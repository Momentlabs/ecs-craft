package interactive

import(
  "fmt"
  "github.com/aws/aws-sdk-go/aws/session"

  // "mclib"
  "github.com/jdrivas/mclib"
)

func doListEnv(sess *session.Session) (error) {
  s, err := mclib.GetServerForName(serverNameArg, currentCluster, sess)
  if err != nil { return err }

  env, ok := s.ServerEnvironment() 
  if !ok {return fmt.Errorf("Failed to find server environment for: %s:%s", currentCluster, serverNameArg)}

  for k, v := range env {
    fmt.Printf("[%s] = %s\n", k, v)
  }
  return nil
}