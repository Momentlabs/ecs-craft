package interactive

import(
  "fmt"
  "os"
  "strings"
  "text/tabwriter"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/route53"

  //
  // Careful now ...
  //
  // "mclib"
  "github.com/jdrivas/mclib"
  
  // "awslib"
  // "github.com/jdrivas/awslib"
)

func doListDNS(sess *session.Session) (err error) {

  records, err := mclib.GetDNSRecords(sess)
  if err != nil { return err }

  displayDNSRecords(records)

  return err
}

func doListProxyDNS(sess *session.Session) (err error) {
  p, err := mclib.GetProxyFromName(proxyNameArg, currentCluster, sess)
  if err != nil { return err }

  records, err := p.DNSRecords()
  if err != nil { return err }

  displayDNSRecords(records)

  return err
}

func displayDNSRecords(records []*route53.ResourceRecordSet) {
  if len(records) == 0 {
    fmt.Printf("No records found.\n")
    return
  } 

  w := tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
  fmt.Fprintf(w, "%sName\tTTL\tRecords%s\n", titleColor, resetColor)
  for _, r := range records {
    fmt.Fprintf(w, "%s%s\t%d\t%s%s\n", nullColor, 
      *r.Name, *r.TTL, dnsResourceString(r.ResourceRecords),  resetColor) 
  }
  w.Flush()
}

func dnsResourceString(rs []*route53.ResourceRecord) (string) {
  if len(rs) == 1 {return *rs[0].Value}
  recordStrings := make([]string, len(rs))
  for _, rr := range rs {
    recordStrings = append(recordStrings, *rr.Value)
  }
  return strings.Join(recordStrings, ",")
}