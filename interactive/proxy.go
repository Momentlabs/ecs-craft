package interactive

import(
  "fmt"
  "os"
  "strings"
  "time"
  "text/tabwriter"
  "github.com/aws/aws-sdk-go/aws/session"

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
    fmt.Fprintf(w, "%sName\tProxy Public Addr\tRcon Private Addr\tStatus\tUptime\tServers\tARN%s\n", titleColor, resetColor)
    if len(proxies) == 0 {
      fmt.Fprintf(w,"%s\tNO PROXIES FOUND ON THIS CLUSTER\n%s", titleColor, resetColor)
    } else {
      for _, p := range proxies {
        serverNames, _ := p.ServerNames()
        dt := dtm[p.TaskArn]
        fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\t%s\t%s\t%s%s\n", nullColor,
          p.Name, p.PublicIpAddress(), p.RconAddress(), dt.LastStatus(), dt.UptimeString(), 
          strings.Join(serverNames, ", "), awslib.ShortArnString(&p.TaskArn), resetColor)
      }
    }
    w.Flush()
   return err
}

func doAttachProxy(sess *session.Session) (error) {
  p, err := mclib.GetProxyFromName(proxyNameArg, currentCluster, sess)
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
  // TODO: Also move to the patttern we've got for servers of ServerSpec to
  // launch from the taskdefinition.
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

// This implements logic to support using a default taskdef with either Random or 
// the DefaultPort configuration. Or any task-definition at all.
func getProxyTaskDef() (td string) {
  td = proxyTaskDefArg
  switch proxyTaskDefArg {
  case "defaultCraftPort":
    td = mclib.BungeeProxyDefaultPortTaskDef
  case "defaultRandomPort":
    td = mclib.BungeeProxyRandomPortTaskDef
  }
  return td
}


// Set AWS_REGION to pass the region automatically
// to everyone. The AWS-SDK looks for this
// env when setting up a session (this also plays well with
// using IAM Roles for credentials). 
// Specifically this makes it possible to log into any container
// and immediatley use craft-config to do on the fly backups etc.
// TODO: Consider moving each of these envs into their own
// separate basic defaults, which can be leveraged into
// the separate proxy and barse verions.
// DRY
func getProxyTaskEnvironment(proxyName, region, bucketName string) awslib.ContainerEnvironmentMap {

  serverName := fmt.Sprintf("%s-hub-server", proxyName)

  cenv := make(awslib.ContainerEnvironmentMap)
  cenv[mclib.BungeeProxyServerContainerName] = map[string]string {
    mclib.RoleKey: mclib.CraftProxyRole,
    mclib.ServerNameKey: proxyName,
    mclib.RconPasswordKey: mclib.ProxyRconPasswordDefault,
    "AWS_REGION": region,
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
    "AWS_REGION": region,
  }

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