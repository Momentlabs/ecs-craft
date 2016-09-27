package interactive

import (
  "testing"
  "github.com/stretchr/testify/assert"

  // "mclib"
  "github.com/jdrivas/mclib"
)

func TestProxyTaskDefSelect(t *testing.T) {

  testPositiveVals := [][]string {
    { defaultProxyTaskDef, mclib.BungeeProxyDefaultPortTaskDef,},
    { "defaultCraftPort", mclib.BungeeProxyDefaultPortTaskDef,},
    { "defaultRandomPort", mclib.BungeeProxyRandomPortTaskDef,},
    { "some_task_def", "some_task_def",},
  }
  for _, v := range testPositiveVals {
    proxyTaskDefArg = v[0]
    assert.Equal(t, v[1], getProxyTaskDef(), "Expecting proxyTaskDef: %s to yeild: %s", v[0], v[1])
  }

  testNegativeVals := [][]string {
    { "defaultCraftPort", mclib.BungeeProxyRandomPortTaskDef,},
    { "defaultRandomPort", mclib.BungeeProxyDefaultPortTaskDef,},
    { "defaultPort", mclib.BungeeProxyDefaultPortTaskDef,},
    { "andomPort", mclib.BungeeProxyRandomPortTaskDef,},
    { "some_task_def", "some_other_task_def",},
  }
  for _, v := range testNegativeVals {
    proxyTaskDefArg = v[0]
    assert.NotEqual(t, v[1], getProxyTaskDef(), "Expecting proxyTaskDef: %s to yeild: %s", v[0], v[1])
  }
 }