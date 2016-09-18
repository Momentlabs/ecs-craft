package interactive

import (
  "fmt"
  "os"
  "time"
  "text/tabwriter"
  "github.com/aws/aws-sdk-go/service/ecs"

  // "awslib"
  "github.com/jdrivas/awslib"
)

func printECSFailures(cluster string, failures []*ecs.Failure) {
  fmt.Printf("%sGot (%d) failures on %s%s\n", failColor, len(failures), cluster, resetColor)
  w := tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
  fmt.Fprintf(w, "%sFailure\tArn%s\n", titleColor, resetColor)
  for _, failure := range failures {
    fmt.Fprintf(w, "%s%s\t%s%s\n", failColor,  *failure.Reason, *failure.Arn, resetColor)
  }
  w.Flush()
}

func printTaskList(tasks []*ecs.Task) {
  w := tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
  fmt.Fprintf(w, "%sCluster\tTaskDef\tStatus\tContainers\tCreated\tArn%s\n", titleColor, resetColor)
  for _, t := range tasks {
    status := "<none>"
    if t.LastStatus != nil {status = *t.LastStatus}
    fmt.Fprintf(w,"%s%s\t%s\t%s\t%s\t%s\t%s%s\n", nullColor, 
      awslib.ShortArnString(t.ClusterArn), awslib.ShortArnString(t.TaskDefinitionArn), status,
      awslib.CollectContainerNames(t.Containers), t.CreatedAt.Local().Format(time.RFC1123), *t.TaskArn,
      resetColor)
  }
}
