package interactive 

import (
  "fmt"
  // "io"
  "os"
  "time"
  "text/tabwriter"
  // "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
  // "github.com/aws/aws-sdk-go/service/ecs"
  // "github.com/aws/aws-sdk-go/service/ec2"
  // "github.com/aws/aws-sdk-go/service/s3"

  //
  // Careful now ...
  // "mclib"
  "github.com/jdrivas/mclib"

)

func doSnapshotListCmd(sess *session.Session) (error) {
  // resp, err := GetSnapshotListForUser(userNameArg)
  snaps, err := mclib.GetSnapshotList(userNameArg, bucketNameArg, sess)
  if err == nil {
    fmt.Printf("%d snapshots in bucket %s as of: %s.\n", len(snaps), bucketNameArg, time.Now().Local())
    tabFlags := tabwriter.StripEscape | tabwriter.DiscardEmptyColumns //| tabwriter.Debug
    w := tabwriter.NewWriter(os.Stdout, 19, 8, 1, ' ', tabFlags)
    fmt.Fprintf(w, "%sUser\tServer\tLastMode\tKey%s\n", emphColor, resetColor)
    for _, a := range snaps {   //  snaps is a list of Archives
      fmt.Fprintf(w, "%s%s\t%s\t%s\t%s%s\n", defaultColor, a.UserName, a.ServerName, a.LastMod(), a.S3Key(), resetColor)
      w.Flush()
    }
  }
  return err
}

