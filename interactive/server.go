package interactive 

import (
  "fmt"
  "os"
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
  ss, err := mclib.NewServerSpec(userName, serverName, region, bucketName, tdArn, sess)
  if err != nil { return err }
  serverEnv := ss.ServerContainerEnv()
  contEnv := ss.ControllerContainerEnv()

  fmt.Printf("%sLaunching new minecraft server:%s\n", successColor, resetColor)
  w := tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
  fmt.Fprintf(w, "%sCluster\tUser\tName\tTask\tRegion\tBucket%s\n", titleColor, resetColor)
  fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\t%s\t%s%s\n", nullColor,
    currentCluster, serverEnv[mclib.ServerUserKey], contEnv[mclib.ServerNameKey], tdArn, 
    contEnv[mclib.ArchiveRegionKey], contEnv[mclib.ArchiveBucketKey], resetColor)
  w.Flush()

  err = launchServer(tdArn, currentCluster, userName, ss, sess)
  return err
}

func doStartServerCmd(sess *session.Session) (err error) {

  userName := userNameArg
  serverName := serverNameArg
  region := DefaultArchiveRegion
  bucketName := DefaultArchiveBucket
  snapshotName := snapshotNameArg
  tdArn := serverTaskArg

  ss, err := mclib.NewServerSpec(userName, serverName, region, bucketName, tdArn, sess)
  if err != nil { return err }

  serverEnv := ss.ServerContainerEnv()
  controllerEnv := ss.ControllerContainerEnv()

  if useFullURIFlag {
    // TODO:
    serverEnv[mclib.WorldKey] = snapshotName
   } else {
    return fmt.Errorf("Please use useFullURIFlag until further notice.")
    // serverEnv[mclib.WorldKey] = mclib.ServerSnapshotURI(DefaultArchiveBucket, userName, serverName, snapshotName)
  }

  fmt.Println("Startig minecraft server:")
  w := tabwriter.NewWriter(os.Stdout, 4, 8, 8, ' ', 0)
  fmt.Fprintf(w, "%sCluster\tUser\tName\tTask\tRegion\tBucket\tWorld%s\n", titleColor, resetColor)
  fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\t%s\t%s\t%s%s\n.", nullColor,
    currentCluster, serverEnv[mclib.ServerUserKey], serverEnv[mclib.ServerNameKey], tdArn, 
    controllerEnv[mclib.ArchiveRegionKey], controllerEnv[mclib.ArchiveBucketKey], serverEnv["WORLD"],
    resetColor)
  w.Flush()

  err = launchServer(tdArn, currentCluster, userName, ss, sess)
  return err
}

// TODO: DONT launch a server if there is already one with the same user and server names.
// TODO: We need to be able to attach a server, on launch, to a running proxy.
// I expect that what we'll do is add an ENV variable to the controller (and might
// as well add it to the server too), pointing somehow to the proxy. Perhaps simply 
// Server/RconPort is enough.
// The controller can then be configured to add the server to the proxy on connection.
// Let's consider a ROLE env variable. 
// TODO: Right the connection betwee the mclib.Server and us is a bit to brittle.
// mclib.Server is really a reflection of what we do here, but it relies heavlily
// on the coordination of TaskDefinition and Container variables that are really being
// managed by mclib. More work needs to be done on proper integration. Perhaps 
// we should be headed toward someting like:
// var s *mclib.Server = mclib.LaunchServer(cluster, userName, serverName, env. sess)
// or mclib.LaunchServerWithTaskDefinition(td, cluster ....)
// func launchServer(taskDefinition, clusterName, userName string, env awslib.ContainerEnvironmentMap, sess *session.Session) (err error) {
func launchServer(tdArn, clusterName, userName string, ss mclib.ServerSpec, 
  sess *session.Session) (err error) {

  serverEnv := ss.ServerContainerEnv()
  controlEnv := ss.ControllerContainerEnv()
  if controlEnv[mclib.ArchiveBucketKey] == "" {
    controlEnv[mclib.ArchiveBucketKey] = DefaultArchiveBucket
  }
  if verbose  || debug {
    fmt.Printf("Making server with container environments: %#v\n", ss.ServerTaskEnv)
  }

  env, err := ss.ContainerEnvironmentMap()
  if err != nil { return err }

  resp, err := awslib.RunTaskWithEnv(clusterName, tdArn, env, sess)
  startTime := time.Now()
  tasks := resp.Tasks
  failures := resp.Failures
  if err == nil {
    newUser := serverEnv[mclib.ServerUserKey]
    serverName := serverEnv[mclib.ServerNameKey]
    fmt.Printf("%s launched %s for %s\n", startTime.Local().Format(time.RFC1123), serverName, newUser)
    if len(tasks) == 1  {
      waitForTaskArn := *tasks[0].TaskArn
      awslib.OnTaskRunning(clusterName, waitForTaskArn, sess, func(taskDescrip *ecs.DescribeTasksOutput, err error) {
        if err == nil {
          s, err  := mclib.GetServer(clusterName, waitForTaskArn, sess)
          if err == nil {
            fmt.Printf("\n%s%s for %s %s:%d is now running (%s). %s\n",
             successColor, s.Name, s.User, s.PublicServerIp, s.ServerPort, time.Since(startTime), resetColor)
          } else {
            fmt.Printf("\n%sServer is now running for user %s on %s. (%s).%s\n",
             successColor, userName, clusterName, time.Since(startTime), resetColor)
          }
        } else {
          fmt.Printf("\n%sErrr on waiting for server to start running: %s%s\n", 
            failColor, err, resetColor)
        }
      })
    } else if len(tasks) > 1 {
      fmt.Printf("%sGot more tasks in response to the launch than expected.%s\n", warnColor, resetColor)
      printTaskList(tasks)
      fmt.Printf("%sNo more updates forthcomming.%s\n", warnColor, resetColor)
    }
    if len(failures) > 0 {
      printECSFailures(clusterName, failures)
    }
  }
  return err
}

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
  var s *mclib.Server
  if s, err = mclib.GetServerForName(serverNameArg, currentCluster, sess); err != nil {
    return err
  }

  var p *mclib.Proxy
  if p, err = mclib.GetProxyByName(proxyNameArg, currentCluster, sess); err != nil {
    return err
  }

  if err = p.AddServer(s); err != nil { return err }
  if err = p.ProxyForServer(s); err != nil { return err }

  return nil
}

func doDescribeAllServersCmd(sess *session.Session) (error) {
  // TODO: This assumes that all tasks in a cluster a minecraft servers.
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

