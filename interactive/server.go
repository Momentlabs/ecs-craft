package interactive 

import (
  "fmt"
  "os"
  "sort"
  "strings"
  "text/tabwriter"
  "time"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/ecs"
  // "github.com/aws/aws-sdk-go/service/ec2"

  //
  // Careful now ...
  //
  // "mclib"
  "github.com/jdrivas/mclib"
  
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



// defaults to restarting a server with state from a world backup as oposed to full server backup.
// TODO: Revist starting from full server vs. world (especially the ops etc.)
func doRestartServerCmd(sess *session.Session) (err error) {
  serverName := serverNameArg
  proxyName := proxyNameArg
  cluster := currentCluster
  tdArn := serverTaskArg
  backup := snapshotNameArg

  // Get set up ....
  oServer, err  := mclib.GetServerFromName(serverName, cluster, sess)
  if err != nil { return fmt.Errorf("Failed to get current server, server not restarted: %s", err) }

  p, err := mclib.GetProxyFromName(proxyName, cluster, sess)
  if err != nil { return fmt.Errorf("Failed to get proxy. Server not restarted: %s", err) }
  // TODO: revist if we want to start a new server even if this is not proxied.
  proxyFound, err := p.IsServerProxied(oServer) 
  if !proxyFound || err != nil {
    if !proxyFound { err = fmt.Errorf("Server (%s) not proxied by (%d). Server not restarted: %s", oServer.Name, p.Name) }
    return fmt.Errorf("Failed to find proxy for server: %s", err)
  }

  if backup == "" {
    bu, err  := oServer.GetLatestWorldSnapshot()
    if err != nil { return fmt.Errorf("Failed to get snapshot to start the server. Server not restarted: %s", err) }
    backup = bu.URI()
  }

  // .... start new server from backup ....
  s, err := startServer(oServer.User, oServer.Name, *oServer.AWSSession.Config.Region, oServer.ArchiveBucket, backup, tdArn, cluster, sess)
  if err != nil {
    fmt.Printf("%sError starting server, new server in unknown state. Server not restarted: %#v", err)
    return err
  }
  fmt.Printf("%sStarting new minecraft server with snapshot %s:%s\n", successColor, backup, resetColor)

  fmt.Printf("%sWaiting for new server to become available.%s\n", warnColor, resetColor)
  nServer, err := mclib.GetServerWait(cluster, *s.TaskArn, sess)
  if err != nil {
    return fmt.Errorf("Failed on waiting for new server to come up: %s. Server not restarted.", err)
  }
  fmt.Printf("%sNew serrver up.%s\n", successColor, resetColor)

  // Remove old server DNS.
  changeInfo, err := p.DetachFromProxyNetwork(oServer)
  if err != nil { return fmt.Errorf("Failed to remove server from DNS. New server up and old server not restarted: %s", err) }
  fmt.Printf("%sRemoved DNS for server %s: %s.%s\n", 
    successColor, oServer.Name,  *changeInfo.Comment, resetColor)
  setAlertOnDnsChangeSync(changeInfo, sess)


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
    fmt.Printf("%sSwitched old server to new server on Proxy.%s\n", successColor, resetColor)
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


  // Kill old server task.
  _, err = awslib.StopTask(cluster, *oServer.TaskArn, sess)
  if err != nil {
    err = fmt.Errorf("Failed to stop original server task. Everything else seemed to work: %s", err)
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
      fmt.Printf("%sStopped task %s at %s\n%s", successColor, 
        awslib.ShortArnString(task.TaskArn), task.StoppedAt.Local(), resetColor)
      if len(task.Containers) > 1 {
        fmt.Printf("There were (%d) conatiners associated with this task.\n", len(task.Containers))
      }
      for i, container := range task.Containers {
        fmt.Printf("%d. Stopped container %s, originally started: %s (%s)\n", i+1, 
          *container.Name, task.StartedAt.Local(), time.Since(*task.StartedAt))
      }
    } else {
      for i, task := range tasks {
        fmt.Printf("%i. Stopped task %s at %s. Started at: %s (%s)\n", i+1, 
          awslib.ShortArnString(task.TaskArn), task.StoppedAt.Local(), task.StartedAt.Local(), time.Since(*task.StartedAt))
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
  fmt.Fprintf(w, "%sUser\tServer\tType\tAddress\tRcon\tServer\tControl\tUptime\tTTS%s\n", titleColor, resetColor)
  if len(servers) == 0 {
    fmt.Fprintf(w,"%s\tNO SERVERS FOUND ON THIS CLUSTER%s\n", titleColor, resetColor)
    w.Flush()
    return nil
  } else {
    for _, s := range servers {
      fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s%s\n", nullColor,
        s.User, s.Name, s.CraftType(), s.PublicServerAddress(), s.RconAddress(), s.ServerContainerStatus(), 
        s.ControllerContainerStatus(), s.UptimeString(), s.DeepTask.TimeToStartString(), 
        resetColor)
    }
  }
  w.Flush()

  return err
}

func doDescribeServerCmd(serverName, clusterName string, sess *session.Session) (error) {

  s, err := mclib.GetServerFromName(serverName, clusterName, sess)
  if err != nil { return err }

  pl, _, err := mclib.GetProxies(clusterName, sess)
  if err != nil { return err }

  var p *mclib.Proxy
  for _, pt := range pl {
    isProxy, err := pt.IsServerProxied(s)
    if err != nil { 
      isProxy = false
      fmt.Printf("Error looking for server proxy: %s/%s", pt.Name, s.Name)
    }
    if isProxy {
      p = pt
      break;
    }
  }

  fqdn := "<not-available>"
  ipAddress := "<not-available>"
  if p != nil {
    dn, err := p.ProxiedServerFQDN(s)
    if err == nil {
      fqdn = dn
      ipAddress = p.PublicIpAddress()
    }
  }

  dt := s.DeepTask

  // Overview stats on server.
  fmt.Printf("%s%s: %s on %s%s\n", titleColor, time.Now().Local().Format(time.RFC1123), s.Name, currentCluster, resetColor)
  w := tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
  fmt.Fprintf(w, "%sUser\tServer\tType\tDNS\tIP\tServer\tControl\tUptime\tTTS%s\n", titleColor, resetColor)
  fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s%s\n", nullColor, 
    s.User, s.Name, s.CraftType(), fqdn, ipAddress, s.ServerContainerStatus(), s.ControllerContainerStatus(), 
    s.UptimeString(), dt.TimeToStartString(), resetColor)
  w.Flush()

  // Task details
  fmt.Printf("\n%sTask%s\n", titleColor, resetColor)
  w = tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
  fmt.Fprintf(w, "%sTaskDefinintion\tARN\tInstnanceID\tTaskRole\tublicIP\tPrivateIP\tNetwork Mode\tStatus%s\n", titleColor, resetColor) 
  roleArn := "<none>"
  if dt.TaskDefinition.TaskRoleArn != nil { roleArn = *dt.TaskDefinition.TaskRoleArn }
  fmt.Fprintf(w,"%s%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s%s\n", nullColor,
    awslib.ShortArnString(dt.TaskDefinition.TaskDefinitionArn), awslib.ShortArnString(s.TaskArn), *dt.GetInstanceID(), roleArn, dt.PublicIpAddress(), dt.PrivateIpAddress(), 
    *dt.TaskDefinition.NetworkMode, dt.LastStatus(), resetColor)
  w.Flush()

  // Volumes
  fmt.Printf("%s\nVolumes%s\n", titleColor, resetColor)
  if len(dt.TaskDefinition.Volumes) > 0 {
    w = tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
    fmt.Fprintf(w, "%sName\tHost Source Path%s\n", titleColor, resetColor)
    for _, v := range dt.TaskDefinition.Volumes {
      fmt.Fprintf(w,"%s%s\t%s%s\n", nullColor, *v.Name, *v.Host.SourcePath, resetColor)
    }
  } else {
    fmt.Printf("No volumes specified.\n")
  }
  w.Flush()

  //
  // Containers
  //

  // Basic status
  envs := make(map[string]map[string]string)
  fmt.Printf("\n%sContainers%s\n", titleColor, resetColor)
  w = tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
  fmt.Fprintf(w, "%sName\tRole\tEssential\tPrivelaged\tStatus\tReason%s\n", titleColor, resetColor)
  for _, c := range dt.Task.Containers {
    env, ok := dt.GetEnvironment(*c.Name)
    role := "<not-available>"
    if ok {
      envs[*c.Name] = env
      if r, ok := env[mclib.RoleKey]; ok {
        role = r
      }
    }
    reason := "<not-available>"
    cdef, ok := awslib.GetContainerDefinition(*c.Name, dt.TaskDefinition)
    var priv bool
    if cdef.Privileged != nil { priv = *cdef.Privileged }
    if c.Reason != nil { reason = *c.Reason }
    fmt.Fprintf(w, "%s%s\t%s\t%t\t%t\t%s\t%s\t%s\n", nullColor, *c.Name, role, *cdef.Essential, priv,
      *c.LastStatus, reason, resetColor)
  }
  w.Flush()

  // Configuration for running.
  w = tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
  fmt.Fprintf(w, "\n%sName\tImage\tEntpryPoint\tCommand%s\n", titleColor, resetColor)
  for _, c := range dt.Task.Containers {
    cdef, _ := awslib.GetContainerDefinition(*c.Name, dt.TaskDefinition)
    fmt.Fprintf(w,"%s%s\t%s\t%s\t%s%s\n", nullColor, *c.Name, *cdef.Image, 
      awslib.JoinStringP(cdef.EntryPoint, ", "), awslib.JoinStringP(cdef.Command, ", "), resetColor)
  }
  w.Flush()


  w = tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
  fmt.Fprintf(w, "\n%sName\tUser\tWorkingDir\tLog\tLog Options%s\n", titleColor, resetColor)
  for _, c := range dt.Task.Containers {
    cdef, _ := awslib.GetContainerDefinition(*c.Name, dt.TaskDefinition)
    workingDirectory := "<none>"
    if cdef.WorkingDirectory != nil { workingDirectory = *cdef.WorkingDirectory }
    logDriver := "<none>"
    if cdef.LogConfiguration != nil && cdef.LogConfiguration.LogDriver != nil { logDriver = *cdef.LogConfiguration.LogDriver }
    user := "<none>"
    if cdef.User != nil { user = *cdef.User }
    options := make(optionMap, 0)
    if cdef.LogConfiguration != nil { options = cdef.LogConfiguration.Options }
    fmt.Fprintf(w,"%s%s\t%s\t%s\t%s\t%s%s\n", nullColor,
      *c.Name, user,  workingDirectory, logDriver, options, resetColor)
  }
  w.Flush()

  // Mounts 
  fmt.Printf("\n%sMount Points:%s\n", titleColor, resetColor)
  var anyMounts bool
  for _, c := range dt.Task.Containers {
    cdef, _ := awslib.GetContainerDefinition(*c.Name, dt.TaskDefinition)
    if len(cdef.MountPoints) > 0 {
      anyMounts = true
      w = tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
      fmt.Fprintf(w, "%sContainer\tSource\tContainer\tReadonly%s\n", titleColor, resetColor)
      for _, mp := range cdef.MountPoints {
        fmt.Fprintf(w,"%s%s\t%s\t%s\t%t%s\n", nullColor, *c.Name, *mp.SourceVolume, *mp.ContainerPath, *mp.ReadOnly, resetColor)
      }
    } 
  }
  w.Flush()
  if !anyMounts {
    fmt.Printf("No mount points specified.\n")
  }

  // Resource Controls
  fmt.Printf("\n%sResource Controls:%s\n", titleColor, resetColor)
  w = tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
  fmt.Fprintf(w, "%sContainer\tCPU\tMemory Limit\tMemory Reservation%s\n", titleColor, resetColor)
  for _, c := range dt.Task.Containers {
    cdef, _ := awslib.GetContainerDefinition(*c.Name, dt.TaskDefinition)
    fmt.Fprintf(w,"%s%s\t%d\t%d\t%d%s\n", nullColor, *c.Name, *cdef.Cpu, *cdef.Memory, *cdef.MemoryReservation, resetColor)
  }
  w.Flush()

  fmt.Printf("%s\nUlimits:%s\n", titleColor, resetColor)
  w = tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
  fmt.Fprintf(w, "%sContainer\tName\tSoft Limit\tHard Limit%s\n", titleColor, resetColor)
  var anyLimits bool
  for _, c := range dt.Task.Containers {
    cdef, _ := awslib.GetContainerDefinition(*c.Name, dt.TaskDefinition)
    if len(cdef.Ulimits) > 0 {
      anyLimits = true
      for _, ul := range cdef.Ulimits {
        fmt.Fprintf(w,"%s%s\t%s\t%d\t%d%s\n", nullColor, *c.Name, *ul.Name, *ul.SoftLimit, *ul.HardLimit, resetColor)
      }
    }
  }
  w.Flush()
  if !anyLimits { fmt.Printf("No ulimits specified.\n")}


  //  Network Bindings
  fmt.Printf("\n%sNetwork Bindings:%s\n", titleColor, resetColor)
  w = tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
  fmt.Fprintf(w, "%sContainer\tIP\tContainer\tHost\tProtocol%s\n", titleColor, resetColor)
  for _, c := range dt.Task.Containers {
    for _, b := range c.NetworkBindings {
      fmt.Fprintf(w,"%s%s\t%s\t%d\t%d\t%s%s\n", nullColor, *c.Name, *b.BindIP, 
        *b.ContainerPort, *b.HostPort, *b.Protocol, resetColor)
    }
  }
  w.Flush()

  // Links
  fmt.Printf("\n%sLinks:%s\n", titleColor, resetColor)
  w = tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
  fmt.Fprintf(w, "%sContainer\tLink%s\n", titleColor, 
    resetColor)
  for _, c := range dt.Task.Containers {
    cdef, _ := awslib.GetContainerDefinition(*c.Name, dt.TaskDefinition)
    for _, l := range cdef.Links {
      fmt.Fprintf(w, "%s%s\t%s%s\n", nullColor, *c.Name, *l, resetColor)
    }
  }
  w.Flush()


  // Environments
  tel := mergeEnvs(envs)
  sort.Sort(ByKeyGroupedByContainer(tel))
  fmt.Printf("%s\nConatiner Environments:%s\n", titleColor, resetColor)
  w = tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
  fmt.Fprintf(w, "%sContainer\tKey\tValue%s\n", titleColor, resetColor)
  for _, te := range tel {
    fmt.Fprintf(w, "%s%s\t%s\t%s%s\n", nullColor, te.Container, te.Key, te.Value, resetColor)
  }
  w.Flush()

  // Per instance metrics
  return err
}

type taskEnvEntry struct {
  Container string
  Key string
  Value string
}

type taskEnvSort struct {
  list []*taskEnvEntry
  less func(tI, tJ *taskEnvEntry) (bool)
}

func (t taskEnvSort) Swap(i, j int) { t.list[i], t.list[j] = t.list[j], t.list[i] }
func (t taskEnvSort) Len() int { return  len(t.list) }
func (t taskEnvSort) Less(i, j int) bool { return t.less(t.list[i], t.list[j]) }

func ByKeyGroupedByContainer(tl []*taskEnvEntry) (taskEnvSort) {
  return taskEnvSort {
    list: tl,
    less: func(tI, tJ *taskEnvEntry) bool {
      if tI.Key == tJ.Key {
        return tI.Container < tJ.Container
      }
      return tI.Key < tJ.Key
    },
  }
}

func mergeEnvs(envs map[string]map[string]string) (el []*taskEnvEntry) {
  el = make([]*taskEnvEntry,0)
  for cName, env := range envs {
    for k, v := range env {
      tee := new(taskEnvEntry)
      tee.Container = cName
      tee.Key = k
      tee.Value = v
      el = append(el, tee)
    }
  }
  return el
}

// Found in ContainerDefinition for the log options.
// Though, this looks like something you miight see throughout aws.
type optionMap map[string]*string
func (om optionMap) String() (s string) {
  for k, v := range om {
    s += fmt.Sprintf("%s: %s, ", k, *v)
  }
  s = strings.TrimSuffix(s,", ")
  return s
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

