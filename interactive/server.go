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

  env := getTaskEnvironment(userNameArg, serverNameArg, DefaultArchiveRegion, DefaultArchiveBucket)
  serverEnv := env[mclib.MinecraftServerContainerName]
  contEnv := env[mclib.MinecraftControllerContainerName]

  fmt.Println("%sLaunching new minecraft server:%s\n", successColor, resetColor)
  w := tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
  fmt.Fprintf(w, "%sCluster\tUser\tName\tTask\tRegion\tBucket%s\n", titleColor, resetColor)
  fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\t%s\t%s%s\n", nullColor,
    currentCluster, serverEnv[mclib.ServerUserKey], contEnv[mclib.ServerNameKey], serverTaskArg, 
    contEnv[mclib.ArchiveRegionKey], contEnv[mclib.ArchiveBucketKey], resetColor)
  w.Flush()

  err := launchServer(serverTaskArg, currentCluster, userNameArg, env, sess)
  return err
}

func doStartServerCmd(sess *session.Session) (err error) {

  env := getTaskEnvironment(userNameArg, serverNameArg, DefaultArchiveRegion, DefaultArchiveBucket)
  controllerEnv := env[mclib.MinecraftControllerContainerName]
  serverEnv := env[mclib.MinecraftServerContainerName]
  if useFullURIFlag {
    // TODO:
    serverEnv[mclib.WorldKey] = snapshotNameArg
  } else {
    serverEnv[mclib.WorldKey] = mclib.SnapshotURI(DefaultArchiveBucket, userNameArg, serverNameArg, snapshotNameArg)
  }

  fmt.Println("Startig minecraft server:")
  w := tabwriter.NewWriter(os.Stdout, 4, 8, 8, ' ', 0)
  fmt.Fprintf(w, "%sCluster\tUser\tName\tTask\tRegion\tBucket\tWorld%s\n", titleColor, resetColor)
  fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\t%s\t%s\t%s%s\n.", nullColor,
    currentCluster, serverEnv[mclib.ServerUserKey], serverEnv[mclib.ServerNameKey], serverTaskArg, 
    controllerEnv[mclib.ArchiveRegionKey], controllerEnv[mclib.ArchiveBucketKey], serverEnv["WORLD"],
    resetColor)
  w.Flush()

  err = launchServer(serverTaskArg, currentCluster, userNameArg, env, sess)
  return err
}

// TODO: DONT launch a server if there is already one with the same user and server names.
// TODO: We need to be able to attach a server, on launch, to a running proxy.
// I expect that what we'll do is add an ENV variable to the controller (and might
// as well add it to the server too), pointing somehow to the proxy. Perhaps simply 
// Server/RconPort is enough.
// The controller can then be configured to add the server to the proxy on connection.
// Let's consider a ROLE env variable. 
func launchServer(taskDefinition, clusterName, userName string, env awslib.ContainerEnvironmentMap, sess *session.Session) (err error) {

  serverEnv := env[mclib.MinecraftServerContainerName]
  controlEnv := env[mclib.MinecraftControllerContainerName]
  if controlEnv[mclib.ArchiveBucketKey] == "" {
    controlEnv[mclib.ArchiveBucketKey] = DefaultArchiveBucket
  }
  if verbose  || debug {
    fmt.Printf("Making server with container environments: %#v\n", env)
  }

  resp, err := awslib.RunTaskWithEnv(clusterName, taskDefinition, env, sess)
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
             successColor, s.Name, s.User, s.ServerIp, s.ServerPort, time.Since(startTime), resetColor)
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
      printECSFailures(failures)
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


func getTaskEnvironment(userName, serverName, region, bucketName string) awslib.ContainerEnvironmentMap {
  cenv := make(awslib.ContainerEnvironmentMap)
  cenv[mclib.MinecraftServerContainerName] = map[string]string {
    mclib.RoleKey: mclib.CraftServerRole,
    mclib.ServerUserKey: userName,
    mclib.ServerNameKey: serverName,
    mclib.OpsKey: userName,
    // "WHITELIST": "",
    mclib.ModeKey: mclib.ModeDefault,
    mclib.ViewDistanceKey: mclib.ViewDistanceDefault,
    mclib.SpawnAnimalsKey: mclib.SpawnAnimalsDefault,
    mclib.SpawnMonstersKey: mclib.SpawnMonstersDefault,
    mclib.SpawnNPCSKey: mclib.SpawnNPCSDefault,
    mclib.ForceGameModeKey: mclib.ForceGameModeDefault,
    mclib.GenerateStructuresKey: mclib.GenerateStructuresDefault,
    mclib.AllowNetherKey: mclib.AllowNetherDefault,
    mclib.MaxPlayersKey: mclib.MaxPlayersDefault,
    mclib.QueryKey: mclib.QueryDefault,
    mclib.QueryPortKey: mclib.QueryPortDefaultString,
    mclib.EnableRconKey: mclib.EnableRconDefault,
    mclib.RconPortKey: mclib.RconPortDefaultString,
    mclib.RconPasswordKey: mclib.RconPasswordDefault, // TODO NO NO NO NO NO NO NO NO NO NO NO NO NO
    mclib.MOTDKey: fmt.Sprintf("A neighborhood kept by %s.", userName),
    mclib.PVPKey: mclib.PVPDefault,
    mclib.LevelKey: mclib.LevelDefault,
    mclib.OnlineModeKey: mclib.OnlineModeDefault,
    mclib.JVMOptsKey: mclib.JVMOptsDefault,
  }

  // Set AWS_REGION to pass the region automatically
  // to the minecraft-controller. The AWS-SDK looks for this
  // env when setting up a session (this also plays well with
  // using IAM Roles for credentials).
  // TODO: Consider moving each of these envs into their own
  // separate basic defaults, which can be leveraged into
  // the separate proxy and barse verions.
  // DRY
  cenv[mclib.MinecraftControllerContainerName] = map[string]string{
    mclib.RoleKey: mclib.CraftControllerRole,
    mclib.ServerUserKey: userName,
    mclib.ServerNameKey: serverName,
    mclib.ArchiveRegionKey: region,
    mclib.ArchiveBucketKey: bucketName,
    mclib.ServerLocationKey: mclib.ServerLocationDefault,
    "AWS_REGION": region,
  }
  return cenv
}

func tasksDescriptionShortString(tasks []*ecs.Task, failures []*ecs.Failure) (s string) {
  switch {
  case len(tasks) == 1:
    task := tasks[0]
    containers := tasks[0].Containers
    switch {
    case len(containers) == 1:
      s += containerShortString(containers[0])
    case len(containers) >= 0:
      s += fmt.Sprintf("%s\n", awslib.ShortArnString(task.TaskDefinitionArn))
      s += fmt.Sprintf("There are (%d) containers assocaited with this task.\n", len(containers))
      for i, c := range containers {
        s+= fmt.Sprintf("%d. %s\n", i+1, containerShortString(c))
      }
    }
  case len(tasks) > 0:
    s += fmt.Sprintf("There are (%d) tasks.\n", len(tasks))
    for i, task := range tasks {
      s += fmt.Sprintf("***** Task %d. %s", i+1, task)
    }
  case len(tasks) == 0:
    s += fmt.Sprintf("No tasks.")
  }
  if len(failures) > 0 {
    s += fmt.Sprintf("There are (%d) failures.\n", len(failures))
    for i, failure := range failures {
      s += fmt.Sprintf("\t%d. %s.\n", i+1, failureShortString(failure))
    }
  }

  return s
}

func containerShortString(container *ecs.Container) (descrip string) {
  descrip += fmt.Sprintf("%s", *container.Name)
  bindings := container.NetworkBindings
  switch {
  case len(bindings) == 1:
    descrip += fmt.Sprintf(" - %s", bindingShortString(bindings[0]))
  case len(bindings) > 1:
    descrip += fmt.Sprintf("\nPorts:\n.")
    for i, bind := range container.NetworkBindings {
      descrip += fmt.Sprintf("\t%d. %s\n", i+1, bindingShortString(bind))
    }
  case len(bindings) == 0:
    descrip += fmt.Sprintf(" - no port bindings.")
  }
  return descrip
}

func bindingShortString(bind *ecs.NetworkBinding) (s string) {
  s += fmt.Sprintf("%s container %d => host %d (%s)",*bind.BindIP, *bind.ContainerPort, *bind.HostPort, *bind.Protocol)
  return s
}

func failureShortString(failure *ecs.Failure) (s string){
  s += fmt.Sprintf("%s - %s", *failure.Arn, *failure.Reason)
  return s
}

// TODO: This currently assumes that all tasks on a cluster are
// server tasks. This is an invalid assumption - though we may find that
// we'll run proxies in a separate cluster. TBD for sure. Regardless of how
// this finally turns out, we need to make server discoery in a cluster
// more robust. See the assana note for more on this: https://app.asana.com/0/150993196087302/177572123631468

func doListServersCmd(sess *session.Session) (err error) { 
  dtm, err := awslib.GetDeepTasks(currentCluster, sess)
  if err != nil {return err}

  if len(dtm) == 0 {
    fmt.Printf("%sThere are no servuers on cluster: %s.%s\n", emphBlueColor, currentCluster, resetColor)
    return nil
  }

  //name uptime ip:port arn server-name STATUS backup-name STATUS
  w := tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
  fmt.Fprintf(w, "%sUser\tServer\tUptime\tAddress\tServer\tControl\tArn%s\n", titleColor, resetColor)
  // for _, dt := range dtm {
  for _, dt := range dtm.DeepTasks(awslib.ByReverseUptime) {
    t := dt.Task
    inst := dt.EC2Instance
    if t != nil && inst != nil {
      cntrs := t.Containers
      address := fmt.Sprintf("%s:%s", *inst.PublicIpAddress, getMinecraftPort(cntrs))
      var uptime time.Duration
      if uptime, err = dt.Uptime(); err != nil { uptime = 0 * time.Millisecond}  // fail silently.
      sC := getContainer(cntrs, mclib.MinecraftServerContainerName)
      sCS := "<no-server>"
      if sC != nil { sCS = fmt.Sprintf("%s", *sC.LastStatus) }
      bC := getContainer(t.Containers, mclib.MinecraftControllerContainerName)
      bCS := "<no-controller>"
      if bC != nil { bCS = fmt.Sprintf("%s", *bC.LastStatus) }
      tArn := awslib.ShortArnString(t.TaskArn)
      cOM := makeContainerOverrideMap(t.Overrides)

      userName, ok  := cOM.getEnv(mclib.MinecraftServerContainerName, mclib.ServerUserKey)
      if !ok {
        userName = "[NONAME]"
      }

      serverName, ok := cOM.getEnv(mclib.MinecraftServerContainerName, mclib.ServerNameKey)
      if !ok {
        serverName = "[NONAME]"
      }

      color := nullColor
      if (sC != nil && !awslib.ContainerStatusOk(sC)) || (bC != nil && !awslib.ContainerStatusOk(bC)) {
        color = failColor
      }

      fmt.Fprintf(w,"%s%s\t%s\t%s\t%s\t%s\t%s\t%s%s\n", color, userName, serverName, 
        shortDurationString(uptime), address, sCS, bCS, tArn, resetColor)
      } else {
        if t != nil {
          tArn := awslib.ShortArnString(t.TaskArn)
          fmt.Fprintf(w,"%s\tCan't find instance for task.\n", tArn)
        }
      }
  }
  w.Flush()

  return err
}

func getContainer(containers []*ecs.Container, name string) (c *ecs.Container) {

  // Find the first one matching the name
  for _, tC := range containers {
    if *tC.Name == name {
      c = tC
      break
    }
  }
  return c
}


func getMinecraftPort(containers []*ecs.Container) (s string) {

  var server *ecs.Container
  for _, container := range containers {
    if *container.Name == mclib.MinecraftServerContainerName { server = container}
  }
  
  if server == nil {
    s = "<none>"
  } else {
    var serverHostPort  *int64
    for _, binding := range server.NetworkBindings {
      if *binding.ContainerPort == mclib.ServerPortDefault {
        serverHostPort = binding.HostPort
      }
    }
    if serverHostPort == nil {
      s = "<none>"
    } else {
      s = fmt.Sprintf("%d", *serverHostPort)
    }
  }
  return s
}


func shortDurationString(d time.Duration) (s string) {
  days := int(d.Hours()) / 24
  hours := int(d.Hours()) % 24
  minutes := int(d.Minutes()) % 60
  if days == 0 {
    s = fmt.Sprintf("%dh %dm", hours, minutes)
  } else {
    s = fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
  }
  return s
}

func allBindingsString(bindings []*ecs.NetworkBinding) (s string) {
  s += "["
  for i, bind := range bindings {
    s += fmt.Sprintf("%d:%d", *bind.ContainerPort, *bind.HostPort)
    if i+1 < len(bindings) {s += ", "}
  }
  s += "]"
  return s
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
      fmt.Printf("Started: %s (%s)\n", task.StartedAt.Local(), shortDurationString(time.Since(*task.StartedAt)))
      fmt.Printf("Status: %s\n", *task.LastStatus)
      fmt.Printf("Task: %s\n", *task.TaskArn)
      fmt.Printf("Task Definition: %s\n", *task.TaskDefinitionArn)
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

func (c ContainerOverrideMap) getEnv(containerName, key string) (s string, ok bool) {
  var co *ecs.ContainerOverride
  if co, ok = c[containerName]; ok {
    ok = false
    env := co.Environment
    for _, kvp := range env {
      if *kvp.Name == key {
        s = *kvp.Value
        ok = true
        break
      }
    }
  }
  return s, ok
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

