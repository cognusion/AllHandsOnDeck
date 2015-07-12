# AllHandsOnDeck
All Hands On Deck (aka "all") is a simple orchestration system for Linux, written in Go. Commands are executed in parallelish, as are workflows (commands within a workflow are executed serially)

**This documentation is pretty awful.**

Basics
======

```bash
go get github.com/cognusion/AllHandsOnDeck
go build -o all
./all --help
Usage of ./all:
  -cmd="": Command to run
  -configs="configs/": Path to the folder where the config files are (*.json)
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
Although it does require a pseudo-terminal, and the user needs permission to execute the commands requests.

To that end, maybe running All on *all* your hosts isn't a grand idea, so perhaps you'll tag the hosts you want to allow the 'uptime' command to run on, with the tag 'uptime', then you can do:
```bash
all --cmd uptime --filter 'Tags == uptime'
```

Configs
=======

So how would you tag your hosts? There aren't any hosts listed on that command!! Relax,
we got this.

All reads all the .json files in the --configs folder (defaults to "configs/" for convenience). Host and Workflow configs stanzas may be smattered about, and will all get meged together when All reads them.

Host
----

Configs specify hosts which can have:
* Address - IP address of the host (optional, if Name is a valid hostname)
* Arch - Architecture of the host (e.g. 'Linux x86_64') (optional)
* Name - Name of the host, if it's a valid DNS hostname, Address may be omitted
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

Filters
=======

So filters are pretty neat-looking, ya? Filters may be specified on the command-line via --filter, or as a value of the "filter" element of a workflow.

Filters are broken up by the boolean 'and' then 'or', so "Tags != noupdate and Tags == yum or Tags == azl and Name != ugly" *first* gets looked at as:

1. Tags != noupdate
2. Tags == yum or Tags == azl
3. Name != ugly"

If the tag list for the host being inspected contains "noupdate", filtering is aborted and the host is skipped. 

Next, if the tag list contains "yum", step 2 is done, otherwise "azl" is looked for. If it too doesn't exist, filtering is aborted and the host is skipped.

Finally, if the name of the host is not ugly, the filter succeeds and this host is added to the list that will get the command or workflow-of-commands executed on it. 

Workflow Special Commands
=========================

One of the things I conveniently ignored in the workflow example above was a particular command: FOR needs-restarting RESTART

This is a work in progress, but what that command does, on some systems, is runs the yum-provided "needs-restarting" command, sanitizes and mangles the results into a list of Well-Known Packages, and then runs "service ... restart" on them (in parallelish).

**I strongly recommend you don't use it.** I do all the time, but I also intimately know the state of my systems, and the ramifications therein.
