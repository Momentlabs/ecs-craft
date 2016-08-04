var gulp = require('gulp');
// requires node version >= 12
var child = require('child_process');
var run = require('gulp-run');
var chalk = require('chalk')
var util = require('gulp-util')
var readline = require('readline')

var gulpProcess;
var verbose = false;
var rl = readline.createInterface({input: process.stdin, output: process.stdin});

gulp.task('test', function() {
  if(verbose) {
    args = ["test", "-v"]
  } else {
    args = ["test"]
  }

  test = child.spawnSync("go", args)
  if(test.status == 0) {
    util.log(chalk.white.bgGreen.bold(' Go Test Successful'));
    if (verbose) {
      var lines = test.stdout.toString().split("\n");
      for (var l in lines) {
        util.log(lines[l]);
      }
    }
  } else {
    util.log(chalk.white.bgRed.bold(" GO Test Failed "))
    var lines = test.stdout.toString().split("\n");
    for (var l in lines) {
      util.log(chalk.red(lines[l]));
    }
  }
  return test
})

gulp.task('build', function() {
  build = child.spawnSync("go", ["install"])
  if(build.status == 0) {
    util.log(chalk.white.bgBlue.bold(' Go Install Successful '));
  } else {
    util.log(chalk.white.bgRed.bold(" GO Install Failed "))
    var lines = build.stderr.toString().split('\n');
    for (var l in lines)
      util.log(chalk.red(lines[l]));
  }
  return build;
});


function doCommand(command) {
  retVal = true
  commands = command.split(" ")
  switch (commands[0]) {
    case 'verbose': 
      if (verbose) {
        verbose = false;
      } else {
        verbose = true;
      }
      util.log("Verbose is now " + verbose.toString())
      break;
    case '': // just eat returns.
      break
    default:
      util.log("Unknown command: ", command)
      retVal = false;
  }
  return retVal;
}

// This is just user interface hackery to get a command line to appear reasonablly.
// really only works if you run: gulp watch --silent
gulp.task('newline', function() {process.stdout.write("\n"); return true;});
gulp.task('prompt', function() {rl.prompt(); return true;});

function doCommandPrompt(answer) {
  doCommand(answer);
  rl.prompt();
}

function commandPromptLoop() {
  rl.setPrompt("command: ");
  rl.on('line', doCommandPrompt);
  rl.prompt();
}

gulp.task('watch', function(){
  gulp.watch('**/*.go', ['newline', 'build', 'test', 'prompt']);
  commandPromptLoop()
})

// Variety of ways explored to get gulp to reload on gulpfile edit.
// In the end went with gulper: npm install -g gulper.
// I can't figure out how to turn off notificationn, but it's a small
// price.
