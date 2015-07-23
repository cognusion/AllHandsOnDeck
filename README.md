# AllHandsOnDeck
All Hands On Deck (aka "all") is a simple agentless orchestration system written in Go, for Linux. You can run it *from* any platform that supports Go (Macs are popular, I hear). Commands are executed in parallelish, as are workflows (commands within a workflow are executed serially);

**This documentation is pretty awful. The in-code documentation is even worse.**

Basics
======

```bash
go get github.com/cognusion/AllHandsOnDeck
go build -o all
./all --help
Usage of ./all:
  -cmd="": Command to run
  -configs="configs/": Path to the folder where the config files are (*.json)
  -configtest=false: Load and parse configs, and exit
  -debug=false: Enable Debug output
  -filter="": Boolean expression to positively filter on Tags
  -sshagent=false: Connect and use SSH-Agent vs. user key
  -sshkey="/home/you/.ssh/id_rsa": If not using the SSH-Agent, where to grab the key
  -sudo=false: Whether to run commands via sudo
  -timeout=60: Seconds before the entire operation times out
  -user="you": User to run as
  -workflow=false: The --cmd is a workflow
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
Although sudo does require a pseudo-terminal, and the user needs permission to execute the commands requests.

To that end, maybe running All on *all* your hosts isn't a grand idea, so perhaps you'll tag the hosts you want to allow the 'uptime' command to run on, with the tag 'uptime', then you can do:
```bash
all --cmd uptime --filter 'Tags == uptime'
```

Configs
=======

So how would you tag your hosts? There aren't any hosts listed on that command up there. Relax, we got this.

All reads all the .json files in the --configs folder (defaults to "configs/" for convenience). Host and Workflow config stanzas may be smattered about, and will all get merged together when All reads them. 

Some utilities like giving configs fancy names so they seem like more than they are. I don't. They're still fancy, though.

Host
----

Configs can specify hosts which can have:
* Address - IP address of the host (optional, if Name is a valid hostname)
* Arch - Architecture of the host (e.g. 'x86_64') (optional)
* Name - Name of the host. If it's a valid DNS hostname, Address may be omitted
* Port - Which port SSH is running on. Defaults to 22.
* Tags - Array of strings which can be used with filters.
* User - A specific user to use when SSHing to this host. Overrides --user param.

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

Filters
=======

So filters are pretty neat-looking, ya? Filters may be specified on the command-line via --filter, or as a value of the "filter" element of a workflow.

Filters are broken up by the boolean 'and' then 'or', so "Tags != noupdate and Tags == yum or Tags == azl and Name != ugly" *first* gets looked at as:

1. Tags != noupdate
2. Tags == yum or Tags == azl
3. Name != ugly

If the tag list for the host being inspected contains "noupdate", filtering is aborted and the host is skipped. 

Next, if the tag list contains "yum", step 2 is done, otherwise "azl" is looked for. If it too doesn't exist, filtering is aborted and the host is skipped.

Finally, if the name of the host is not ugly, the filter succeeds and this host is added to the list that will get the command or workflow-of-commands executed on it. 

Workflow Special Commands
=========================


FOR
---

    FOR list ACTION

One of the things I conveniently ignored in the workflow example above was a particular command: FOR needs-restarting RESTART

This is a work in progress, but what that command does, on some systems, is runs the yum-provided "needs-restarting" command, sanitizes and mangles the results into a list of Well-Known Packages, and then runs "service ... restart" on them (in parallelish).

**I strongly recommend you don't use it.** I do all the time, but I also intimately know the state of my systems, and the ramifications therein.

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

### RAND

    RAND(n)
    
Additionally, to aid in making temp folders, etc. there is a special nugget to create random
alpha-numeric strings of set length "n". This may be embedded anywhere in a SET string. e.g.

```bash
SET %TMPDIR% /tmp/specialRAND(8)folder
```

### S3

    S3(s)
    
You can specify an AWS S3 URL (e.g. s3://bucketname/some/file/some/where) and if you have an AWS access and secret key specified in the configs, a time-expiring (60 minute) URL will automagically be created.

```bash
SET %MYURL% S3(s3://mybucket/myfile.mov)
```

Some Things You Haven't Asked Yet
=================================

### Semicolons & Sessions

All is a remote shell interface, as such pretty much anything you can do on a single shell (probably BASH) line, you can do in a single All command. You can use semicolons to separate statements just like you can in a shell, and they will act how you would expect.

```bash
# hostname;uptime
myserver
23:20  up 57 mins, 2 users, load averages: 1.37 1.37 1.27
```

Every command in All is executed as a unique session to the remote host, so if you need to ensure same-session execution of commands, use semicolons. 


### Timeouts

Timeouts in All may not work how you expect them to. They are not per-command, or per-session, or per-host, or per-workflow: They are per-All-operation. So if you specify a 5 second timeout, and are asking 1000 hosts to execute 16 commands in a workflow, they've all got 5 seconds before All bails, and who-knows-what ends up happening on-systems. For that reason, a "mintimeout" is available in each workflow, to automatically bump the timeout if it isn't already. This should generally be generously high.


Forward, Ho
===========

All was written for specific purposes 2013-2014, and is being ground-up rewritten to take advantage of new Go tech, lessons learned from 2 years of using it, and lessons learned from writing lots of Go - better Go - since then. As such, All as it is here isn't complete yet. Additionally, there are some things I want to add that would have been very difficult in with the old code base. The TODO list, In no particular order:

1. scp-able helper files (quazi-agents) (almost unnecessary with new workflow)
2. Configurable output (JSON, XML, whatevs)
3. allsh, the All shell
4. Moar "Workflow Special Commands"
5. Command output analysis
6. Option to fail Workflow command on stderr content
7. Better visibility into what is happening, including a "dryrun" facility

**Pull requests are welcome**. If you're serious about wanting to hack at something here, please reach out. I may/probably have pointers or even stub code related to these.

Also, feel free to add to this list, via feature requests, or their more better brethren: pull requests. :)