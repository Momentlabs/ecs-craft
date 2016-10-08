package interactive 

import (
  "fmt"
  "os"
  "strings"
  "text/tabwriter"
  "time"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/ecs"
  "github.com/aws/aws-sdk-go/service/ec2"

  //
  // Careful now ...
  //
  "mclib"
  // "github.com/jdrivas/mclib"
  
  // "awslib"
  "github.com/jdrivas/awslib"
)


//
// Server commands
//


const (
  // TODO: Probably want to set this up as a command line option at some point.
  // NOTE: We're explicitly NOT using the commaind line awsRegionArg here.
  // The archive and the rest of interacting with AWS should be separate. At 
  // least for now. Though I expect that as of this moment, this tool
  // doesn't work with using a different region for the archive as 
  // where you're running. But it fairly easily could.
  DefaultArchiveRegion = "us-east-1"
  DefaultArchiveBucket = "craft-config-test"
)


func doLaunchServerCmd(sess *session.Session) (error) {
  userName := userNameArg
  serverName := serverNameArg
  region := DefaultArchiveRegion
  bucketName := DefaultArchiveBucket
  tdArn := serverTaskArg
  cluster := currentCluster

  ss, err := mclib.NewServerSpec(userName, serverName, region, bucketName, cluster, tdArn, sess)
  if err != nil { return err }

  s, err := launchServer(ss, sess)
  if err == nil {
    displayServer(s)
  }
  return err
}

func doStartServerCmd(sess *session.Session) (err error) {

  userName := userNameArg
  serverName := serverNameArg
  region := DefaultArchiveRegion
  bucketName := DefaultArchiveBucket
  snapshotName := snapshotNameArg
  tdArn := serverTaskArg
  cluster := currentCluster

  // startServer calls launchServer, which handles reporting on multiple tasks.
  s, err := startServer(userName, serverName, region, bucketName, snapshotName, tdArn, cluster,sess)
  if err == nil {
    displayServer(s)
  }
  return err
}

// Used just as it's launched.
func displayServer(s *mclib.Server) {
  serverEnv, ok := s.ServerEnvironment()
  if !ok { fmt.Printf("Failed to get Server environment.")}
  contEnv, ok := s.ControllerEnvironment()
  if !ok { fmt.Printf("Failed to get Controller environment.")}
  fmt.Printf("%sLaunching new minecraft server:%s\n", successColor, resetColor)
  w := tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
  fmt.Fprintf(w, "%sCluster\tUser\tName\tTask\tRegion\tBucket%s\n", titleColor, resetColor)
  fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\t%s\t%s%s\n", nullColor,
    s.ClusterName, serverEnv[mclib.ServerUserKey], contEnv[mclib.ServerNameKey], 
    *s.DeepTask.TaskDefinition.TaskDefinitionArn, contEnv[mclib.ArchiveRegionKey], 
    contEnv[mclib.ArchiveBucketKey], resetColor)
  w.Flush()
  if debug {
    fmt.Printf("Server Environment:")
    for k, v := range serverEnv {
      fmt.Printf("[%s] = %s", k, v)
    }
    fmt.Printf("\nController Environment:\n")
    for k, v := range contEnv {
      fmt.Printf("[%s] = %s", k, v)
    }
    fmt.Printf("\n")
  }
}



func doRestartServerCmd(sess *session.Session) (err error) {
  serverName := serverNameArg
  proxyName := proxyNameArg
  cluster := currentCluster
  tdArn := serverTaskArg
  backup := snapshotNameArg

  // Get set up ....
  oServer, err  := mclib.GetServerFromName(serverName, cluster, sess)
  if err != nil { return fmt.Errorf("Failed to get server, server not restarted: %s", err) }

  if backup == "" {
    bs, err := oServer.GetLatestWorldSnapshot()
    if err != nil { return fmt.Errorf("Failed to get latest world, server not restarted: %s", err)}
    backup = bs.URI()
    fmt.Printf("Using snapshot: %s.\n", backup)
  }

  p, err := mclib.GetProxyFromName(proxyName, cluster, sess)
  if err != nil { return fmt.Errorf("Failed to get proxy. Server not restarted: %s", err) }
  // TODO: revist if we want to start a new server even if this is not proxied.
  proxyFound, err := p.IsServerProxied(oServer) 
  if !proxyFound || err != nil {
    if !proxyFound { err = fmt.Errorf("Server (%s) not proxied by (%d). Server not restarted: %s", oServer.Name, p.Name) }
    return fmt.Errorf("Failed to find proxy for server: %s", err)
  }

  changeInfo, err := p.DetachFromProxyNetwork(oServer)
  if err != nil { return fmt.Errorf("Failed to remove server from DNS. Server not restarted: %s", err) }
  fmt.Printf("%sRemoved DNS for server %s: %s.%s\n", 
    successColor, oServer.Name,  *changeInfo.Comment, resetColor)
  setAlertOnDnsChangeSync(changeInfo, sess)


  // .... start new server from backup ....
  s, err := startServer(oServer.User, oServer.Name, *oServer.AWSSession.Config.Region, oServer.ArchiveBucket, backup, tdArn, cluster, sess)
  if err != nil {
    fmt.Printf("%sError starting server, server in unknown state, DNS for old server removed: %#v", err)
    return err
  }
  fmt.Printf("%sStarting new minecraft server with snapshot %s:%s\n", successColor, backup, resetColor)

  fmt.Printf("Waiting for new server to become available.")
  nServer, err := mclib.GetServerWait(cluster, *s.TaskArn, sess)
  if err != nil {
    return fmt.Errorf("Failed to get a Server Object from server task: %s. New server launced old server still in place, but no DNS.", err)
  }

  // .... Unproxy old server ....
  successMessages := make([]string,0)
  errorMessages := make([]string, 0)
  serr := p.StopProxyForServer(oServer)
  if serr == nil {
    successMessages = append(successMessages,"Proxy no longer acts as proxy for Server")
  } else {
    em := fmt.Sprintf("Failed to stop proxy for server: %s", serr)
    errorMessages = append(errorMessages, em)
  }

  rerr := p.UpdateServerAccess(nServer)
  if rerr == nil {
    successMessages = append(successMessages,"Server access update to new server.")
  } else {
    em := fmt.Sprintf("Failed to switch access from old to new server on Proxy: %s", rerr)
    errorMessages = append(errorMessages, em)
  }

  if serr != nil || rerr != nil {
    var successMessage string
    if len(successMessages) == 1 {
      successMessage = successMessages[0]
    } else { 
      successMessage = strings.Join(successMessages, ", ")
    }

    var errorMessage string
    if len(errorMessages) == 1 {
      errorMessage = errorMessages[0]
    } else { 
      errorMessage = strings.Join(errorMessages, ", ")
    }

    em := fmt.Sprintf("%s, but %s.\nServer (%s) has not been killed. But has no DNS record.",
      errorMessage, successMessage, oServer.Name)
    return fmt.Errorf(em)
  } else {
    fmt.Printf("%sSwitch old server to new server on Proxy.%s\n", successColor, resetColor)
  }

  sFQDN, ci, err := p.AttachToProxyNetwork(nServer)
  if err != nil {
    err = fmt.Errorf("Failed to update New Server DNS to proxy: %s. However, Server access added to proxy and proxy will forward.", err)
    return  err
  }
  fmt.Printf("%sNew Server has DNS to proxy: %s%s\n", successColor, sFQDN, resetColor)
  setAlertOnDnsChangeSync(ci, sess)

  err = p.StartProxyForServer(nServer) 
  if err != nil {
    return fmt.Errorf("Failed to make proxy forward for server (forcedHost): %s", err)
  }
  fmt.Printf("%sProxy will now forward connections for server.%s\n", successColor, resetColor )


  // .... kill old server task.
  _, err = awslib.StopTask(cluster, *oServer.TaskArn, sess)
  if err != nil {
    err = fmt.Errorf("Failed to stop original serer task. Everything else seemed to work: %s", err)
  }
  fmt.Printf("%sOld server sucesfullly terminated.%s\n", successColor, resetColor)

  fmt.Printf("%sServer Restarted.%s\n", successColor, resetColor)
  serverEnv, ok  := s.ServerEnvironment()
  if !ok { fmt.Printf("Failed to get the server Environment.") }
  controllerEnv, ok := s.ControllerEnvironment()
  if !ok { fmt.Printf("Failed to get the controller Environment.") }
  w := tabwriter.NewWriter(os.Stdout, 4, 8, 8, ' ', 0)
  fmt.Fprintf(w, "%sCluster\tUser\tName\tTask\tRegion\tBucket\tWorld%s\n", titleColor, resetColor)
  fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\t%s\t%s\t%s%s\n", nullColor,
    currentCluster, serverEnv[mclib.ServerUserKey], serverEnv[mclib.ServerNameKey], tdArn, 
    controllerEnv[mclib.ArchiveRegionKey], controllerEnv[mclib.ArchiveBucketKey], serverEnv["WORLD"],
    resetColor)
  w.Flush()

  return err
}



// Set up the environment to start the server from a snapshot.
func startServer(un, sn, region, bn, snapshotName, tdArn, clusterName string, 
  sess *session.Session) (s *mclib.Server, err error) {

  ss, err := mclib.NewServerSpec(un, sn, region, bn, clusterName, tdArn, sess)
  if err != nil { return s, err }

  serverEnv := ss.ServerContainerEnv()
  serverEnv[mclib.WorldKey] = snapshotName
  s, err = launchServer(ss, sess)
  return s, err
}


// TODO: Figure out if this is an issue: don't launch a server if there is already 
// one with the same user and server names. Probably only really matters in the case
// of proxing. That said, shouldn't we just prevent this?
func launchServer(ss mclib.ServerSpec, sess *session.Session) (s *mclib.Server, err error) {

  startTime := time.Now()
  s, err = ss.LaunchServer()
  if err != nil { 
    fmt.Printf("%sFail in launch server. %#v%s", failColor, err, resetColor)
    return s, err 
  }

  if err == nil {
    awslib.OnTaskRunning(s.ClusterName, *s.TaskArn, sess, func(taskDescrip *ecs.DescribeTasksOutput, err error) {
      if err == nil {
        // go get the most recent data.
        ns, err  := mclib.GetServer(s.ClusterName, *s.TaskArn, sess)
        if err == nil {
          fmt.Printf("\n%s%s for %s %s:%d is now running (%s) on cluster: %s. %s\n",
           successColor, ns.Name, ns.User, ns.PublicServerIp, ns.ServerPort, time.Since(startTime), ns.ClusterName, resetColor)
        } else {
          fmt.Printf("\n%sServer is now running for user %s on %s. (%s).%s\n",
           successColor, s.Name, s.ClusterName, time.Since(startTime), resetColor)
        }
      } else {
        fmt.Printf("\n%sErrr on waiting for server to start running: %s%s\n", 
          failColor, err, resetColor)
      }
    })
  }

  return s, err
}

// TODO: This should get moved to mclib.
func doTerminateServerCmd(sess *session.Session) (error) {

  _, err := awslib.StopTask(currentCluster, serverTaskArnArg, sess)
  if err != nil { return fmt.Errorf("terminate server failed: %s", err) }

  fmt.Printf("Server Task stopping: %s.\n", awslib.ShortArnString(&serverTaskArnArg))
  awslib.OnTaskStopped(currentCluster, serverTaskArnArg,  sess, func(stoppedTaskOutput *ecs.DescribeTasksOutput, err error) {
    if stoppedTaskOutput == nil {
      fmt.Printf("Task %s stopped.\nMissing Task Object.\n", serverTaskArnArg)
      return
    }
    tasks := stoppedTaskOutput.Tasks
    failures := stoppedTaskOutput.Failures
    if len(tasks) > 1 {
      fmt.Printf("%sExpected 1 task in OnStop got (%d)%s\n", warnColor, len(tasks), resetColor)
    }
    if len(failures) > 0 {
      fmt.Printf("%sReceived (%d) failures in stopping task.%s\n", failColor, len(failures), resetColor)
    }
    if len(tasks) == 1 {
      task := tasks[0]
      fmt.Printf("%sStopped task %s at %s\n%s", successColor, awslib.ShortArnString(task.TaskArn), task.StoppedAt.Local(), resetColor)
      if len(task.Containers) > 1 {
        fmt.Printf("There were (%d) conatiners associated with this task.\n", len(task.Containers))
      }
      for i, container := range task.Containers {
        fmt.Printf("%d. Stopped container %s, originally started: %s (%s)\n", i+1, *container.Name, task.StartedAt.Local(), time.Since(*task.StartedAt))
      }
    } else {
      for i, task := range tasks {
        fmt.Printf("%i. Stopped task %s at %s. Started at: %s (%s)\n", i+1, awslib.ShortArnString(task.TaskArn), task.StoppedAt.Local(), task.StartedAt.Local(), time.Since(*task.StartedAt))
      }
    }
    if len(failures) > 0 {
      w := tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
      fmt.Fprintf(w, "Arn\tFailure\n")
      for _, failure := range failures {
        fmt.Fprintf(w, "%s\t%s\n", *failure.Arn, *failure.Reason)
      }
      w.Flush()
    }
  })

  return nil
}

func doListServersCmd(sess *session.Session) (err error) { 
  servers, err := mclib.GetServers(currentCluster, sess)
  if err != nil {return err}

  fmt.Printf("%s%s servers on %s%s\n", titleColor, 
    time.Now().Local().Format(time.RFC1123), currentCluster, resetColor)
  w := tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
  fmt.Fprintf(w, "%sUser\tServer\tType\tAddress\tRcon\tServer\tControl\tUptime\tArn%s\n", titleColor, resetColor)
  if len(servers) == 0 {
    fmt.Fprintf(w,"%s\tNO SERVERS FOUND ON THIS CLUSTER%s\n", titleColor, resetColor)
    w.Flush()
    return nil
  } else {
    for _, s := range servers {
      fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s%s\n", nullColor,
        s.User, s.Name, s.CraftType(), s.PublicServerAddress(), s.RconAddress(), s.ServerContainerStatus(), 
        s.ControllerContainerStatus(), s.UptimeString(), awslib.ShortArnString(s.TaskArn), 
        resetColor)
    }
  }
  w.Flush()

  return err
}

func doServerProxyCmd(sess *session.Session) (err error) {

  s, err := mclib.GetServerFromName(serverNameArg, currentCluster, sess)
  if err != nil { return err }
  p, err := mclib.GetProxyFromName(proxyNameArg, currentCluster, sess) 
  if err != nil { return err }


  if err = p.AddServerAccess(s); err != nil { return err }
  sFQDN, ci, err := p.AttachToProxyNetwork(s)
  if err != nil {
    err = fmt.Errorf("Failed to update Server DNS to proxy: %s. However, Server access added to proxy.", err)
    return  err
  }

  err = p.StartProxyForServer(s)
  if err == nil {
    fmt.Printf("%sServer added to proxy. New DNS for %s%s\n", successColor, sFQDN, resetColor)
    setAlertOnDnsChangeSync(ci, sess)
  }
  return err
}

func doServerUnProxyCmd(sess *session.Session) (err error) {

  s, err := mclib.GetServerFromName(serverNameArg, currentCluster, sess)
  if err != nil { return err }
  p, err := mclib.GetProxyFromName(proxyNameArg, currentCluster, sess)
  if err != nil { return err }


  successMessages := make([]string,0)
  errorMessages := make([]string, 0)

  changeInfo, derr := p.DetachFromProxyNetwork(s)
  if derr == nil {
    successMessages = append(successMessages, "DNS for server removed")
  } else {
    em := fmt.Sprintf("Failed to remove DNS for server: %s", derr)
    errorMessages = append(errorMessages,em)
  }

  serr := p.StopProxyForServer(s)
  if serr == nil {
    successMessages = append(successMessages,"Proxy no longer acts as proxy for Server")
  } else {
    em := fmt.Sprintf("Failed to stop proxy for server: %s", serr)
    errorMessages = append(errorMessages, em)
  }

  rerr := p.RemoveServerAccess(s)
  if rerr == nil {
    successMessages = append(successMessages,"Server access removed from Proxy")
  } else {
    em := fmt.Sprintf("Failed to remove server access from Proxy: %s", rerr)
    errorMessages = append(errorMessages, em)
  }


  if derr != nil || serr != nil || rerr != nil {
    var successMessage string
    if len(successMessages) == 1 {
      successMessage = successMessages[0]
    } else { 
      successMessage = strings.Join(successMessages, ", ")
    }
    var errorMessage string
    if len(errorMessages) == 1 {
      errorMessage = errorMessages[0]
    } else { 
      errorMessage = strings.Join(errorMessages, ", ")
    }
    err = fmt.Errorf("%s, but %s", errorMessage, successMessage)
  } else {
    fmt.Printf("%sRemoved server from proxy and DNS for server%s\n", successColor, resetColor)
  }

  if derr == nil {
    setAlertOnDnsChangeSync(changeInfo, sess)
  }

  return err
}


func doDescribeAllServersCmd(sess *session.Session) (error) {
  dtm, err := awslib.GetDeepTasks(currentCluster, sess)
  if err != nil {return err}

  taskCount := 0
  for _, dtask := range dtm {
    taskCount++
    task := dtask.Task
    ec2Inst := dtask.EC2Instance
    containers := task.Containers
    if task != nil && ec2Inst != nil {
      fmt.Printf("=========================\n")
      fmt.Printf("%s", longDeepTaskString(task, ec2Inst))
      if len(containers) > 1 {
        fmt.Printf("There were (%d) containers associated with this task.\n", len(containers))
      }
      coMap := makeContainerOverrideMap(task.Overrides)
      for i, container := range containers {
        fmt.Printf("* %d. Container Name: %s\n", i+1, *container.Name)
        fmt.Printf("Network Bindings:\n%s", networkBindingsString(container.NetworkBindings))
        fmt.Printf("%s\n", overrideString(coMap[*container.Name], 3))
      }

    }
    if dtask.Failure != nil {
      fmt.Printf("Task failure - Reason: %s, Resource ARN: %s\n", *dtask.Failure.Reason, *dtask.Failure.Arn)
    }
    if dtask.CIFailure != nil {
      fmt.Printf("ContainerInstance failure - Reason: %s, Resource ARN: %s\n", *dtask.CIFailure.Reason, *dtask.CIFailure.Arn)
    }
  }

  return nil
}

func longDeepTaskString(task *ecs.Task, ec2Inst *ec2.Instance) (s string) {
      fmt.Printf("Task Definition: %s\n", awslib.ShortArnString(task.TaskDefinitionArn))
      fmt.Printf("Instance IP: %s\n", *ec2Inst.PublicIpAddress)
      fmt.Printf("Instance ID: %s\n", *ec2Inst.InstanceId)
      fmt.Printf("Instance Type: %s\n", *ec2Inst.InstanceType)
      fmt.Printf("Location: %s\n", *ec2Inst.Placement.AvailabilityZone)
      fmt.Printf("Public DNS: %s\n", *ec2Inst.PublicDnsName)
      fmt.Printf("Started: %s (%s)\n", task.StartedAt.Local(), awslib.ShortDurationString(time.Since(*task.StartedAt)))
      fmt.Printf("Status: %s\n", *task.LastStatus)
      fmt.Printf("Task: %s\n", *task.TaskArn)
      fmt.Printf("Task Definition: %s\n", *task.TaskDefinitionArn)
      return s
}

func bindingShortString(bind *ecs.NetworkBinding) (s string) {
  s += fmt.Sprintf("%s container %d => host %d (%s)",*bind.BindIP, *bind.ContainerPort, *bind.HostPort, *bind.Protocol)
  return s
}

func failureShortString(failure *ecs.Failure) (s string){
  s += fmt.Sprintf("%s - %s", *failure.Arn, *failure.Reason)
  return s
}

func networkBindingsString(bindings []*ecs.NetworkBinding) (s string) {
  for i, b := range bindings {
    s += fmt.Sprintf("\t%d  %s\n", i+1, bindingShortString(b))
  }
  return s
}


// Overrides by container.
type ContainerOverrideMap map[string]*ecs.ContainerOverride

func makeContainerOverrideMap(to *ecs.TaskOverride) (ContainerOverrideMap) { 
  coMap := make(ContainerOverrideMap)
  for _, co := range to.ContainerOverrides {
    coMap[*co.Name] = co
  }
  return coMap
}

func overrideString(co *ecs.ContainerOverride, perLine int) (s string) {
  if perLine == 0 {perLine = 1}
  command := "<EMPTY>"
  if co.Command != nil {command = commandString(co.Command)}
  s += fmt.Sprintf("Command: %s\n", command)
  s += fmt.Sprintf("Environment: ")
  for i, kvp := range co.Environment {
    s += fmt.Sprintf("%s = %s", *kvp.Name, *kvp.Value)
    if len(co.Environment) != i+1 { s += ", " }
    if (i+1)%perLine == 0 {s += "\n"}
  }

  return s
}

func commandString(c []*string) (s string) {
  for _, com := range c {
    s+= *com + " "
  }
  return s
}

func doDescribeServerCmd() (error) {
  fmt.Printf("Describe server for user \"%s\" in cluster \"%s\".\n", userNameArg, currentCluster)
  return nil
}

