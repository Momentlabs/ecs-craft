package interactive

import(
  "fmt"
  "os"
  "text/tabwriter"
  "time"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/route53"
  "github.com/jdrivas/awslib"
  "github.com/aws/aws-sdk-go/service/ecs"

  // "mclib"
  "github.com/jdrivas/mclib"
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

// Wait and notify on proxy task running.
// Then on success create DNS for the proxy.
// TODO: wants refactoring too much going on here. 
// perhaps the thing to do is turn this into a wait-on-with-notify taking a function
// to execute when the dns is ready. See for integration with above.
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
          fmt.Printf("%sNot attaching proxy to network! Check to see if task created.\n%s", failColor, resetColor)
          return
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