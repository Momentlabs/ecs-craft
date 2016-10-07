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
  am, err := mclib.GetArchives(userNameArg, bucketNameArg, sess)
  sMap := am[userNameArg]
  if err == nil {
    headerString := fmt.Sprintf("%s%s: %d servers for %s in bucket [%s].%s", 
      emphBlueColor, time.Now().Local().Format(time.RFC1123), 
      len(sMap), userNameArg, bucketNameArg,  resetColor)

    fmt.Printf("%s\n",headerString)
    tabFlags := tabwriter.StripEscape | tabwriter.DiscardEmptyColumns //| tabwriter.Debug
    w := tabwriter.NewWriter(os.Stdout, 19, 8, 1, ' ', tabFlags)
    fmt.Fprintf(w, "%sUser\tServer\tType\tLastMode\tKey%s\n", emphColor, resetColor)
    for _, snaps := range sMap {   //  snaps is a list of Archives
      sort.Sort(mclib.ByLastMod(snaps))
      for _, a := range snaps {
        fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\t%s%s\n", defaultColor, a.UserName, a.ServerName, 
          a.Type, a.LastMod().Format(time.RFC1123), a.S3Key(), resetColor)
      }
      w.Flush()
    }
  }
  return err
}

