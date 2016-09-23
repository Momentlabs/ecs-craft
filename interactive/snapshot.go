package interactive 

import (
  "fmt"
  // "io"
  "os"
  "sort"
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

func doArchiveListCmd(sess *session.Session) (error) {
  // resp, err := GetSnapshotListForUser(userNameArg)
  am, err := mclib.GetArchives(userNameArg, bucketNameArg, sess)
  snaps := am[userNameArg]
  if err == nil {
    sort.Sort(mclib.ByLastMod(snaps))
    headerString := fmt.Sprintf("%s%s: %d snapshots for %s in bucket [%s].%s", 
      emphBlueColor, time.Now().Local().Format(time.RFC1123), 
      len(snaps), userNameArg, bucketNameArg,  resetColor)

    fmt.Printf("%s\n",headerString)
    tabFlags := tabwriter.StripEscape | tabwriter.DiscardEmptyColumns //| tabwriter.Debug
    w := tabwriter.NewWriter(os.Stdout, 19, 8, 1, ' ', tabFlags)
    fmt.Fprintf(w, "%sUser\tServer\tType\tLastMode\tKey%s\n", emphColor, resetColor)
    for _, a := range snaps {   //  snaps is a list of Archives
      fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\t%s%s\n", defaultColor, a.UserName, a.ServerName, 
        a.Type, a.LastMod().Format(time.RFC1123), a.S3Key(), resetColor)
      w.Flush()
    }
    // Put a status line at the bottom if there are a few lines.
    if len(snaps) > 8 {
      fmt.Printf("%s\n",headerString)
    }
  }
  return err
}

