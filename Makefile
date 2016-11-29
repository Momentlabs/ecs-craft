repo := ecs-craft
aws_profile = momentlabs

prog := ecs-craft
release_dir := release
builds := darwin_build linux_build
darwin_target := $(release_dir)/$(prog)_darwin_amd64
linux_target := $(release_dir)/$(prog)_linux_amd64 

help:
	@echo make check \# Looks for imports in source files for the local version of mclib, awslib.
	@echo make release-build \# Creates the binaries: $(binaries)
	@echo make new-release version=v0.0.2 description="This is an early release." \# creates a release on github.
	@echo make release-publish version=v0.0.2 \# pushes the binaries to the github release.

check:
	@ if grep -e '^[[:space:]]*\"awslib\"' *go interactive/*.go ; then \
		echo "Fix the library refrence."; \
		exit -1; \
	else echo "awslib checked o.k."; \
	fi
	@ if grep -e '^[[:space:]]*\"mclib\"' *go interactive/*.go; then \
		echo "Fix the library refrence."; \
		exit -1; \
	else echo "mclib checked o.k."; \
	fi

clean:
	rm -rf release

# Only define these variables for the release build.
$(darwin_target) $(linux_target) : now := $(shell date +%s)
$(darwin_target) $(linux_target) : timeflag := -X $(prog)/version.unixtime=$(now)
$(darwin_target) $(linux_target) : hash := $(shell git rev-parse HEAD)
$(darwin_target) $(linux_target) : hashflag := -X $(prog)/version.githash=$(hash)
$(darwin_target) $(linux_target) : env := production
$(darwin_target) $(linux_target) : envflag := -X $(prog)/version.environ=$(env)
$(darwin_target) $(linux_target) : ld_args := $(envflag) $(hashflag) $(timeflag)

$(darwin_target) :
	GOOS=darwin GOARC=amd64 go build "-ldflags=$(ld_args)" -o $(release_dir)/$(prog)_darwin_amd64

$(linux_target) :
	GOOS=linux GOARC=amd64 go build "-ldflags=$(ld_args)" -o $(release_dir)/$(prog)_linux_amd64

darwin_build : $(darwin_target)

# This is a docker build to get a linux target because of golang cgo dependency in os.user
linux_build :
	docker-compose up --build

release-build: check $(builds)

# TODO: Consider doing some git tagging and building in a file for description.
new-release: clean release-build
	@echo creating release on github, version: ${version}: $(description)
	github-release release -u Momentlabs -r $(prog) -t ${version} -d "${description}"
	github-release upload -u Momentlabs -r $(prog) -t ${version} -n $(prog)_linux_amd64 -f $(release_dir)/$(prog)_linux_amd64
	github-release upload -u Momentlabs -r $(prog) -t ${version} -n $(prog)_darwin_amd64 -f $(release_dir)/$(prog)_darwin_amd64

publish-release: release-build
	github-release upload -u Momentlabs -r $(prog) -t ${version} -n $(prog)_linux_amd64 -f $(release_dir)/$(prog)_linux_amd64
	github-release upload -u Momentlabs -r $(prog) -t ${version} -n $(prog)_darwin_amd64 -f $(release_dir)/$(prog)_darwin_amd64

