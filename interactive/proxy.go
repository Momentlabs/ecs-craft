package interactive

import(
  "fmt"
  "os"
  "time"
  "text/tabwriter"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/ecs"
  "github.com/aws/aws-sdk-go/service/route53"

  // "mclib"
  "github.com/jdrivas/mclib"

  // "awslib"
  "github.com/jdrivas/awslib"
)

func doListProxies(sess *session.Session) (error) {
    proxies, dtm, err := mclib.GetProxies(currentCluster, sess)
    fmt.Printf("%s%s proxies on %s%s\n", titleColor, 
      time.Now().Local().Format(time.RFC1123), currentCluster, resetColor)
    w := tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
    fmt.Fprintf(w, "%sName\tProxy Public Addr\tRcon Private Addr\tStatus\tUptime\tARN%s\n", titleColor, resetColor)
    if len(proxies) == 0 {
      fmt.Fprintf(w,"%s\tNO PROXIES FOUND ON THIS CLUSTER\n%s", titleColor, resetColor)
    } else {
      for _, p := range proxies {
        dt := dtm[p.TaskArn]
        fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\t%s\t%s%s\n", nullColor,
          p.Name, p.PublicIpAddress(), p.RconAddress(), dt.LastStatus(), dt.UptimeString(), 
          awslib.ShortArnString(&p.TaskArn), resetColor)
      }
    }
    w.Flush()
   return err
}

func doAttachProxy(sess *session.Session) (error) {
  p, err := mclib.GetProxyByName(currentCluster, proxyNameArg, sess)
  if err == nil {
    domainName, changeInfo, err := p.AttachToNetwork()
    if err == nil {
      status := "----"
      if changeInfo.Status != nil { status = *changeInfo.Status}
      t := "-------"
      if changeInfo.SubmittedAt != nil { t = changeInfo.SubmittedAt.Local().Format(time.RFC1123) }
      id := "-------"
      if changeInfo.Id != nil { id = *changeInfo.Id }
      w := tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
      fmt.Fprintf(w, "%sDNS\tPublic IP\tDNS Status\tDNS Time\tDNS ID%s\n", titleColor, resetColor)
      fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\t%s%s\n", nullColor, domainName, p.PublicProxyIp, status, t, id, resetColor)
      w.Flush()
      setAlertOnDnsChangeSync(changeInfo, sess)
    }
  }

  return err
}

// TODO: Much of this needs to move to mclib.
func doLaunchProxy(sess *session.Session) (error) {

  // Get these from the UI for now.
  // TODO: want to do some form of config for this,
  // though this may stand so it can be overridden by the UI ......
  proxyName := proxyNameArg
  bucketName := DefaultArchiveBucket
  proxyTaskDef := getProxyTaskDef()
  clusterName := currentCluster

  env := getProxyTaskEnvironment(proxyName,DefaultArchiveRegion,bucketName)
  start := time.Now()
  resp, err := awslib.RunTaskWithEnv(clusterName, proxyTaskDef, env, sess)
  if err != nil { return err }

  if len(resp.Failures) > 0 {
    printECSFailures(clusterName, resp.Failures)
    return fmt.Errorf("Received %d failiures on launch.", len(resp.Failures))
  }

  proxyEnv := env[mclib.BungeeProxyServerContainerName]
  contEnv := env[mclib.BungeeProxyHubControllerContainerName]
  fmt.Printf("%s%s launching new Proxy and Hub:%s\n", 
    successColor, start.Local().Format(time.RFC1123), resetColor)
  w := tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
  fmt.Fprintf(w, "%sCluster\tName\tTask\tRegion\tBucket%s\n", titleColor, resetColor)
  fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\t%s%s\n", nullColor,
    clusterName, proxyEnv[mclib.ServerNameKey], proxyTaskDef, 
    contEnv[mclib.ArchiveRegionKey], contEnv[mclib.ArchiveBucketKey], resetColor)
  w.Flush()

  tasks := resp.Tasks
  if len(tasks) == 1 {
    setUpProxyWaitAlerts(clusterName, *tasks[0].TaskArn, sess)
  } else {
    fmt.Printf("%sGot more tasks in response to the launch than expected.%s\n", warnColor, resetColor)
    printTaskList(tasks)
    fmt.Printf("%sNo more updates forthcomming.%s\n", warnColor, resetColor)
  }
  return nil
}

// TODO: This could be made slightly more robust by returning
// an error and noting if there were conflicts on the command line:
// e.g. We actively selected conflicting port-plan and a task-def.
// Probalby not worth the trouble.
func getProxyTaskDef() (string) {
  switch proxyPortArg {
    case proxyDefaultPort: return mclib.BungeeProxyDefaultPortTaskDef
    case proxyRandomPort: return mclib.BungeeProxyRandomPortTaskDef
    case proxyUnselectedPort: return proxyTaskDefArg
  }
  return proxyTaskDefArg
}

// Communicate DNS update status.
func setUpProxyWaitAlerts(clusterName, waitTask string, sess *session.Session) {
  fmt.Printf("%sWaiting for containers to be available before attaching to network. Updates will follow.%s\n", warnColor, resetColor)
  awslib.OnTaskRunning(clusterName, waitTask, sess,
    func(taskDecrip *ecs.DescribeTasksOutput, err error) {
      if err == nil {
        p, err := mclib.GetProxy(clusterName, waitTask, sess)
        if err == nil {
          fmt.Printf("\n%sProxy Task Running, server comming up.%s\n", titleColor, resetColor)
          w := tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
          fmt.Fprintf(w, "%sProxy\tProxy IP\tRcon IP\tTask%s\n", titleColor, resetColor)
          fmt.Fprintf(w, "%s%s\t%s\t%s\t%s%s\n", successColor,
            p.Name, p.PublicIpAddress(), p.RconAddress(), 
            awslib.ShortArnString(&p.TaskArn), resetColor)
          w.Flush()
        } else {
          fmt.Printf("%sAlerted that a new proxy task is running, but there was an error getting details: %s%s\n",
            warnColor, err, resetColor)
        }

        fmt.Printf("%sAttaching to network ....%s", warnColor, resetColor)
        domainName, changeInfo, err := p.AttachToNetwork()
        if err == nil {
          fmt.Printf("%s Attached. %s: %s => %s. It may take some time for the DNS to propocate.%s\n",
            successColor, changeInfo.SubmittedAt.Local().Format(time.RFC1123), domainName, p.PublicProxyIp,
            resetColor)
          setAlertOnDnsChangeSync(changeInfo, sess)
        } else {
          fmt.Printf("%s Failed to attach to DNS: %s%s\n", failColor, err, resetColor)
        }
      } else {
        fmt.Printf("%sError on proxy task running alert: %s%s\n", failColor, err, resetColor)
      }
  })
}

// Set's up a wait for resource records sets change, and alerts on the results.
// Returns immediately, but will print an alert to stdout on change or error.
func setAlertOnDnsChangeSync(changeInfo *route53.ChangeInfo, sess *session.Session) {
  fmt.Printf("%sDNS changes propgating through the network. Will alert when synched.\n%s", warnColor, resetColor)
  awslib.OnDNSChangeSynched(changeInfo.Id, sess, func(ci *route53.ChangeInfo, err error) {
    fmt.Printf("\n%sDNS Change Synched %s%s\n", titleColor, 
      time.Now().Local().Format(time.RFC1123), resetColor)
    w := tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
    fmt.Fprintf(w, "%sStatus\tSubmitted\tElapsed\tComment%s\n", titleColor, resetColor)
    color := nullColor
    if err == nil {
      color = successColor
    } else {
      color = warnColor
      fmt.Fprintf(w, "%sError: %s%s\n", failColor, err, resetColor)
    }
    fmt.Fprintf(w,"%s%s\t%s\t%s\t%s%s\n", color,
      *ci.Status, ci.SubmittedAt.Local().Format(time.RFC822), 
      awslib.ShortDurationString(time.Since(*ci.SubmittedAt)), *ci.Comment, resetColor)
    w.Flush()
  })
}

func getProxyTaskEnvironment(proxyName, region, bucketName string) awslib.ContainerEnvironmentMap {

  serverName := fmt.Sprintf("%s-hub-server", proxyName)

  cenv := make(awslib.ContainerEnvironmentMap)
  cenv[mclib.BungeeProxyServerContainerName] = map[string]string {
    mclib.RoleKey: mclib.CraftProxyRole,
    mclib.ServerNameKey: proxyName,
    mclib.RconPasswordKey: mclib.ProxyRconPasswordDefault,
  }

  cenv[mclib.BungeeProxyHubServerContainerName] = map[string]string {
    mclib.RoleKey: mclib.CraftHubServerRole,
    mclib.ServerUserKey: proxyName,
    mclib.ServerNameKey: serverName,
    // mclib.OpsKey: userName,
    // "WHITELIST": "",
    mclib.ModeKey: mclib.ProxyHubModeDefault,
    mclib.ViewDistanceKey: mclib.ProxyHubViewDistanceDefault,
    mclib.SpawnAnimalsKey: mclib.ProxyHubSpawnAnimalsDefault,
    mclib.SpawnMonstersKey: mclib.ProxyHubSpawnMonstersDefault,
    mclib.SpawnNPCSKey: mclib.ProxyHubSpawnNPCSDefault,
    mclib.ForceGameModeKey: mclib.ProxyHubForceGameModeDefault,
    mclib.GenerateStructuresKey: mclib.ProxyHubGenerateStructuresDefault,
    mclib.AllowNetherKey: mclib.ProxyHubAllowNetherDefault,
    mclib.MaxPlayersKey: mclib.ProxyHubMaxPlayersDefault,
    mclib.QueryKey: mclib.ProxyHubQueryDefault,
    mclib.QueryPortKey: mclib.ProxyHubQueryPortDefault,
    mclib.EnableRconKey: mclib.ProxyHubEnableRconDefault,
    mclib.RconPortKey: mclib.ProxyHubRconPortDefault,
    mclib.RconPasswordKey: mclib.ProxyHubRconPasswordDefault, // TODO NO NO NO NO NO NO NO NO NO NO NO NO NO
    mclib.MOTDKey: fmt.Sprintf("The gateway to %s.", proxyName),
    mclib.PVPKey: mclib.ProxyHubPVPDefault,
    mclib.LevelKey: mclib.ProxyHubLevelDefault,
    mclib.OnlineModeKey: mclib.ProxyHubOnlineModeDefault,
    mclib.JVMOptsKey: mclib.ProxyHubJVMOptsDefault,
  }

  // Set AWS_REGION to pass the region automatically
  // to the minecraft-controller. The AWS-SDK looks for this
  // env when setting up a session (this also plays well with
  // using IAM Roles for credentials).
  // TODO: Consider moving each of these envs into their own
  // separate basic defaults, which can be leveraged into
  // the separate proxy and barse verions.
  // DRY
  cenv[mclib.BungeeProxyHubControllerContainerName] = map[string]string {
    mclib.RoleKey: mclib.CraftControllerRole,
    mclib.ServerNameKey: serverName,
    mclib.ArchiveRegionKey: region,
    mclib.ArchiveBucketKey: bucketName,
    mclib.ServerLocationKey: mclib.ServerLocationDefault,
    "AWS_REGION": region,
  }

  return cenv
}