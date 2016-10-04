package interactive

import(
  "fmt"
  "os"
  "text/tabwriter"
  "time"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/route53"
  "github.com/jdrivas/awslib"
)


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