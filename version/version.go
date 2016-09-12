package version

import (
  "fmt"
  "strconv"
  "time"
)

//
// Change the version number here.
//
const(
  major = 0
  minor = 0
  dot = 1
)
// These will get set by ldFlags during the build.
var (
  // buildstamp string
  githash string
  environ string
  unixtime string
)

func init() {
  ut, err := strconv.ParseInt(unixtime, 10, 64)
  if err != nil {
    ut = 0
  }
  buildTime := time.Unix(ut, 0)
  Version = AppVersion{
    major: major,
    minor: minor,
    dot: dot,
    githash: githash,
    environ: environ,
    buildStamp: buildTime,
  }
}


type  AppVersion struct {
    major int
    minor int
    dot int
    githash string
    environ string
    buildStamp time.Time
}

var Version AppVersion

func (AppVersion) String() string {
  return fmt.Sprintf("Version: %d.%d.%d (%s) %s [%s]", 
    Version.major, Version.minor, Version.dot,
    Version.environ, Version.buildStamp.Local().Format(time.RFC1123), Version.githash)
}







