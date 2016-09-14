package interactive

import(
  "fmt"
  "os"
  "time"
  "text/tabwriter"
  "github.com/aws/aws-sdk-go/aws/session"
  // "awslib"
  "github.com/jdrivas/awslib"
)

func doListClusters(sess *session.Session) (error) {
  clusters, err := awslib.GetAllClusterDescriptions(sess)
  clusters.Sort(awslib.ByReverseActivity)
  if err != nil {
    fmt.Printf("doQuit: Error getting cluster data: %s\n", err)
  } else {
    fmt.Printf("%s%s%s\n", titleColor, time.Now().Local().Format(time.RFC1123), resetColor)
    w := tabwriter.NewWriter(os.Stdout, 4, 10, 2, ' ', 0)
    fmt.Fprintf(w, "%sName\tStatus\tInstances\tPending\tRunning%s\n", titleColor, resetColor)
    for _, c := range clusters {
      instanceCount := *c.RegisteredContainerInstancesCount
      color := nullColor
      if instanceCount > 0 {color = infoColor}
      fmt.Fprintf(w, "%s%s\t%s\t%d\t%d\t%d%s\n", color, *c.ClusterName, *c.Status, 
        instanceCount, *c.PendingTasksCount, *c.RunningTasksCount, resetColor)
    }      
    w.Flush()
  }
  return err
}

func doClusterStatus(sess *session.Session) (error) {
  fmt.Printf("%sCommand not implemented.%s\n", warnColor, resetColor)
  return nil
}