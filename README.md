# AllHandsOnDeck
All Hands On Deck (aka "all") is a simple agentless orchestration system written in Go, for Linux. You can run it *from* any platform that supports Go (Macs are popular, I hear). Commands are executed via SSH in parallelish, as are workflows (commands within a workflow are executed serially);

Basics
======

All allows you to execute arbitrary "commands" on hosts. You can also group "commands" into "workflows", which can be pretty complicated. 

"Filters" can be applied to "workflows" and/or specified on the command-line. CLI filters are applied first.

```bash
go get -d github.com/cognusion/AllHandsOnDeck
cd $GOPATH/src/github.com/cognusion/AllHandsOnDeck
go test
go build -o all

./all --help
Usage of ./all:
  -awshosts
    	Get EC2 hosts and tags from AWS API
  -awsregions string
    	Comma-delimited list of AWS Regions to check if --awshosts is set
  -bar
    	If outputting to a logfile, display a progress bar (default true)
  -cmd string
    	The command to run
  -configdump
    	Load and parse configs, dump them to output and exit
  -configs string
    	Path to the folder where the config files are (*.json) (default "configs/")
  -configtest
    	Load and parse configs, and exit
  -debug
    	Enable Debug output
  -debuglogfile string
    	Output debugs to a logfile, instead of standard error
  -dnf
    	Use dnf instead of yum for some commands
  -dryrun
    	If you want to go through the motions, but never actually SSH to anything
  -errorlogfile string
    	Output errors to a logfile, instead of standard error
  -filter string
    	Boolean expression to positively filter on host elements (Tags, Name, Address, Arch, User, Port, etc.)
  -format string
    	Output format. One of: text, json, or xml (default "text")
  -listhosts
    	List the hostnames and addresses and exit
  -listworkflows
    	List the workflows and exit
  -logfile string
    	Output to a logfile, instead of standard out (enables progressbar to screen)
  -max int
    	Specify the maximum number of concurrent commands to execute. Set to 0 to make a good guess for you (default 0)
  -quiet
    	Suppress most-if-not-all normal output
  -sleep string
    	Duration to sleep between host iterations (e.g. 32ms or 1s) (default "0ms")
  -sshagent
    	Connect and use SSH-Agent vs. user key
  -sshkey string
    	If not using the SSH-Agent, where to grab the key (default "/home/m/.ssh/id_rsa")
  -sudo
    	Whether to run commands via sudo
  -timeout int
    	Seconds before the entire operation times out (default 60)
  -user string
    	User to run as (default "M")
  -vars string
    	Comma-delimited list of variables to pass in for use in workflows, sometimes
  -wave int
    	Specify which "wave" this should be applied to
  -workflow string
    	The workflow to run
```

At it's simplest, All will execute the specified command on all your hosts, via your SSH key.

```bash
all --cmd uptime
```
Using a running ssh-agent is as hard as adding --sshagent to the command string, which allows you to support multiple keys, etc.
```bash
all --cmd uptime --sshagent
```
It may be necessary to run some commands via sudo, which is similarly trivial.
```bash
all --cmd 'uptime; whoami' --sudo
```
Although sudo does require a pseudo-terminal, and the user needs permission to execute the commands requested.

To that end, maybe running All on *all* your hosts isn't a grand idea, so perhaps you'll tag the hosts you want to allow the 'uptime' command to run on, with the tag 'uptime', then you can do:
```bash
all --cmd uptime --filter 'Tags == uptime'
```

Configs
=======

So how would you tag your hosts? There aren't any hosts listed on that command up there. Relax, we got this.

All reads all of the .json files in the --configs folder (defaults to "configs/" for convenience). Host and Workflow config stanzas may be smattered about, and will all get merged together when All reads them. 

Additionally, if you use Amazon Web Services EC2, you can use the live inventory via their API: Just need your keys.

Some utilities like giving configs fancy names like "recipes" or "playbooks" so they seem like more than they are. I don't. These configs are still fancy, though.

Host
----

Configs can specify hosts which can have:
* Address - IP address of the host (optional, if Name is a valid DNS hostname) (AWS: Private IP Address)
* Arch - Architecture of the host (e.g. 'x86_64') (optional) (AWS: Architecture)
* Loc - Location of the system (e.g. 'Denver', or 'Rack 12', or whatever) (optional) (AWS: Availability Zone)
* Wave - If you want to run commands in waves, you can specify an affinity number >0. May be filtered using --wave CLI param, and/or standard filters (optional) (AWS: Value of EC2 tag "wave")
* Name - Name of the host. If it's a valid DNS hostname, Address may be omitted (AWS: Value of EC2 tag "Name")
* Offline - True if the host is offline and should be skipped, else omitted or false (AWS: True if the state is not "running")
* Port - Which port SSH is running on. Defaults to 22. (AWS: Value of EC2 tag "sshport")
* Tags - Array of strings which can be used with filters. (AWS: See note about AWS Tags below)
* User - A specific user to use when SSHing to this host. Overrides --user param.  (AWS: Value of EC2 tag "sshuser")

```json
{
	"hosts": [
		{
			"name": "mymongobox",
			"address": "10.0.2.2",
			"port": 22,
			"tags": ["azl", "mongo-2.4","prod"]
		},
		{
			"name": "mywebbox",
			"address": "10.0.2.1",
			"port": 22,
			"tags": ["azl", "httpd","ssl","prod"],
			"user": "root"
		}
	]
}
```

### AWS Tags

The "tags" array will be populated with all of the EC2 tags (EXCEPT the ones previously noted that are used to fill in other fields) in the format of "key|value" unless only the "key" is defined, then it will just be "key". **Keep this in mind when filtering**!!

Workflow
--------

Configs may also declare "workflows". A workflow is simply a named list of commands. 

```json
{
	"workflows": [
		{
			"name": "uptime",
			"commands": [
				"uptime"
				]
		}
	]
}
```
Workflows are quite powerful, and allow you to specify:
* Filter - An optional filter string. More on this later
* MinTimeout - An optional number of seconds a workflow should at the very least run for
* Name - Whatever you want to call the workflow
* Sudo - If this workflow must run via sudo, set this to 'true'
* Commands - An ordered list of commands
* CommandBreaks - An optional ordered list of booleans specifying whether an error executing the corresponding command should break the workflow. By default, always true.

```json
{
	"workflows": [
		{
			"name": "yumupdateall",
			"filter": "Tags != noupdate and Tags == yum or Tags == azl and Name != ugly",
			"sudo": true,
			"mintimeout": 600,
			"commands": [
				"yum clean all",
				"yum update -y",
				"FOR needs-restarting RESTART"
			],
			"commandbreaks": [
				true,
				true,
				false
			]
		}
	]
}
```
It is worth noting that each command in a workflow is executed in order, serially, and atomically. Thus if you "cd" in one command, don't expect that in a subsequent command cwd will be where you left it. If you *must* do such things (there is probably a better way to do what you're thinking of), chain multiple commands with semicolons, e.g.

```bash
"cd /tmp; mkdir X; cd X; somecommand"
```

Misc
----

Some things are clunky on the CLI, or shouldn't be passed that way, or we're simply lazy and want to make sure that whenever we run anything, that thing is set the way we want. That's what a _Misc_ is for. Miscs override CLI params if set.

```json
{
	"miscs": [
		{
			"name": "awsaccess_key",
			"value": "ABCDEFG"
		},
		{
			"name": "awsaccess_secretkey",
			"value": "123456789"
		},
		{
			"name": "useawshosts",
			"value": "true"
		},
		{
			"name": "dontrestart-processes",
			"value": "udevd,mongod,tomcat,java,dirsrv,ns-slapd"
		},
		{
			"name": "usesshagent",
			"value": "true"
		},
		{
			"name": "maxexecs",
			"value": "30"
		}
	]
}
```

### awsaccess_key

Along with _awsaccess_secretkey_ below, these are used for Amazon Web Services operations that need credentials. Currently just creating S3 time-token URLs when using the _S3()_ workflow special command.

### awsaccess_secretkey

Along with _awsaccess_key_ above, these are used for Amazon Web Services operations that need credentials. Currently just creating S3 time-token URLs when using the _S3()_ workflow special command.

### aws_regions

Along with all the other _awsaccess_ bits, this is a comma-delimited list of AWS EC2 Regions
to act on if either _--awshosts_ or _useawshosts_ are used. This is the equivalient of _--awsregions_.
```json
	{
		"name": "aws_regions",
		"value": "us-west-1,eu-central-1"
	}
```

### debugoutputlog

Specifies where you want debug logging to go if you use _--debug_ (versus stderr).

### dontrestart-processes

If you use the _FOR list ACTION_ workflow special command, this slightly misnamed config allows you to specify a comma-delimited list of processes you don't want to _ACTION_ under any circumstances.
```json
    {
		"name": "dontrestart-processes",
		"value": "udevd,mongod,tomcat,java,dirsrv,ns-slapd"
	}
```

### erroroutputlog

Specifies where you want error logging to go (versus stderr).

### maxexecs

The system default for maximum execution is 0 (educated guess), and if you always want that to be something different, it's obnoxious to specify it on the CLI all the time. Set this instead:
```json
	{
		"name": "maxexecs",
		"value": "30"
	}
```

### outputformat

The default _-format_ is "text", and if you always want that to be something different, it's obnoxious to specify it on the CLI all the time. Set this instead:
```json
	{
		"name": "outputformat",
		"value": "json"
	}
```

Values are text, json, or xml.

### outputlog

Specifies where you want regular output to go (versus stdout).

### useawshosts

If you always want to use the AWS EC2 inventory, set this instead:
```json
	{
		"name": "useawshosts",
		"value": "true"
	}
```


### usesshagent

If you always want to use an SSH agent, it's obnoxious to specify it on the CLI all the time. Set this instead:
```json
	{
		"name": "usesshagent",
		"value": "true"
	}
```

### usednf
If you always want to use dnf instead of yum, it's obnoxious to specify it on the CLI all the time. Set this instead:
```json
	{
		"name": "usednf",
		"value": "true"
	}
```

Filters
=======

So filters are pretty neat-looking, ya? Filters may be specified on the command-line via --filter, and/or as a value of the "filter" element of a workflow.

Filters are broken up by the boolean 'and' then 'or', so "Tags != noupdate and Tags == yum or Tags == azl and Name != ugly" *first* gets looked at as:

1. Tags != noupdate
2. Tags == yum or Tags == azl
3. Name != ugly

If the tag list for the host being inspected contains "noupdate", filtering is aborted and the host is skipped. 

Next, if the tag list contains "yum", step 2 is done, otherwise "azl" is looked for. If it too doesn't exist, filtering is aborted and the host is skipped.

Finally, if the name of the host is not ugly, the filter succeeds and this host is added to the list that will get the command or workflow-of-commands executed on it.

Since v2.1, filters also support limited fuzzy matching via _~=_ ("kinda equal") and _~!_ ("kinda not") operators. **Currently, it is a simple substring match, but may evolve in the future.**

AWS Tags
--------

The "tags" array will be populated with all of the EC2 tags (EXCEPT the ones previously noted that are used to fill in other fields) in the format of "key|value" unless only the "key" is defined, then it will just be "key". **Keep this in mind when filtering**!!

Workflow Special Commands
=========================

SLEEP
-----

    SLEEP duration

Sure, you could waste an SSH connection to have the remote system sleep, but it's probably smarter to just sleep locally a lot of the time. The argument is a Go "duration" ala "1s" or "10m" or "3ms".

```bash
SLEEP 5s
```

QUIET
-----

    QUIET command

Sometimes a command's output is inconsequential. Leading it with _QUIET_ will suppress output of that one command when the workflow is post-processing. 

FOR
---

    FOR list ACTION

One of the things I conveniently ignored in the workflow example above was a particular command: FOR needs-restarting RESTART

This is a work in progress, but what that command does, on some systems, is runs the yum-provided "needs-restarting" command, sanitizes and mangles the results into a list of Well-Known Packages, and then runs "service ... restart" on them (in parallelish).

**I strongly recommend you don't use it.** I do all the time, but I also intimately know the state of my systems, and the ramifications therein. You've been warned.

ACTION is currently one of: START, STOP, RESTART, STATUS, and "list" is either the keyword "needs-restarting", as described above, or a space-separated list of inits to act on, e.g.

```bash
FOR httpd tomcat mysql STOP
FOR mysql tomcat httpd START
FOR mongod STATUS
```

SET
---

    SET %varname% "Some String Value"

Variables, macros, what-have-you are what makes programming worth doing. All is worth doing
too, so it needs those. In any workflow, you can put a SET command in lieu of a "real" command, to create a variable bound to the workflow, to be used later in any non-SET command.

SET commands are evaluated _once_, before any executions take place. So, for example, if your command list looks like:

```bash
SET %TMPDIR% /tmp/specialRAND(16)
mkdir %TMPDIR%
```

Every host will have the same folder created.

### RAND

    RAND(n)
    
Additionally, to aid in making temp folders, etc. there is a special nugget to create random
alpha-numeric strings of set length "n". This may be embedded anywhere in a SET string. e.g.

```bash
SET %TMPDIR% /tmp/specialRAND(8)folder
```

### S3

    S3(s)
    
You can specify an AWS S3 URL (e.g. s3://bucketname/some/file/some/where) and if you have an AWS access and secret key specified in the misc configs, a time-expiring (60 minute) URL will automagically be created.

```bash
SET %MYURL% S3(s3://mybucket/myfile.mov)
```

Some Things You Haven't Asked Yet
=================================

## Running Without Filters

Running a command or workflow with All, with no filters, is telling All, quite literally, "do this, on all the things". Filters limit the scope of execution. No filters: No limits.

## Semicolons & Sessions

All is a remote shell interface, and as such pretty much anything you can do on a single shell (probably BASH) line, you can do in a single All command. You can use semicolons to separate statements just like you can in a shell, and they will act how you would expect.

```bash
# hostname;uptime
myserver
23:20  up 57 mins, 2 users, load averages: 1.37 1.37 1.27
```

Every command in All is executed as a unique session to the remote host, so if you need to ensure same-session execution of commands, use semicolons. 


## Timeouts

Timeouts in All may not work how you expect them to. They are not per-command, or per-session, or per-host, or per-workflow: They are per-All-operation. So if you specify a 5 second timeout, and are asking 1000 hosts to execute 16 commands in a workflow, with a _-max_ of 15, they've all got 5 seconds before All bails, and who-knows-what ends up happening on-systems. For that reason, a "mintimeout" is available in each workflow, to automatically bump the timeout if it isn't already. This should generally be generously high.

## Concurrency

One thing to remember, especially with regards to the timeouts, is that All does launch commands and workflows in parallel*ish* against all of the relevant hosts. Delays connecting to or getting returns from one or more hosts do not hold up others (unless your concurrent host operations are being gated, see Maxexecs, below), although they will delay the operation.

### Maxexecs

There is a gating mechanism that keeps the number of simultaneous operations to a sane limit in order to prevent exhausting socket/open file resources on the running host (I'm looking at you, MacOS). _-max_ on the CLI or the misc _maxexecs_ controls how many can be executing at a time (by way of a semaphore). By default this is set to 0, which causes All to make a pretty decent guess by taking the OS limit for open files, subtracting how many files are currently open by the process, and dividing all that by twice the number of commands in the requested workflow. **If this is resulting in "out of file" errors please submit an issue report!** Of course, you can downlimit this to save yourself some cycles. You can also change your open file limit by using ulimit, ala _ulimit -n 1024_ or whatever.

## Silence

If you want no output whatsoever because reasons:

```bash
all -logfile=/dev/null -errorlogfile=/dev/null -bar=false

```

## Windows

All builds on Windows platforms, however there are some quirks:

* Windows builds aren't generally tested beyond assuring they build clean
* The _maxexecs_ autodetection is completely unavailable, and set to _GOMAXPROCS_ if 
autodetection is requested

## AWS EC2 "Waves"

The "wave" facility is nice if you want explicit control of which set of hosts is being impacted, however in AWS EC2, assuming you're doing it right, you can use Availability Zones (which are populated into the _Loc_ Host field) in lieu of waves:

```bash
all --filter "Loc == us-east-1a" ...
all --filter "Loc == us-east-1b" ...

 # Hit an entire Region (note the tilde operator)
all --filter "Loc ~= us-west-2" ...
```


Forward, Ho
===========

All was written for specific purposes 2013-2014, and is being ground-up rewritten to take advantage of new Go tech, lessons learned from 2 years of using it, and lessons learned from writing lots of Go - better Go - since then. As such, All as it is here isn't complete yet. Additionally, there are some things I want to add that would have been very difficult in with the old code base. The TODO list, In no particular order:

1. scp-able helper files (quazi-agents) (almost unnecessary with new workflow)
3. allsh, the All shell
4. Moar "Workflow Special Commands"
6. Option to fail Workflow command on stderr content

**Pull requests are welcome**. If you're serious about wanting to hack at something here, please reach out. I may/probably have pointers or even stub code related to these.

Also, feel free to add to this list, via feature requests, or their more better brethren: pull requests. :)
