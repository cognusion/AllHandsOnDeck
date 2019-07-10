package main

import (
	"strings"
	"testing"
)

// Test arrays for needsRestartingMangler
var (
	emptyTest = []string{}
	noneTest  = strings.Split(`2137 : /sbin/dhclient -q -lf /var/lib/dhclient/dhclient-eth0.leases -pf /var/run/dhclient-eth0.pid eth0
2492 : /sbin/mingetty /dev/tty2
2207 : irqbalance
2218 : dbus-daemon --system
2494 : /sbin/mingetty /dev/tty3
2496 : /sbin/mingetty /dev/tty4
1 : /sbin/init
2498 : /sbin/mingetty /dev/tty5
2490 : /sbin/mingetty /dev/tty1
2503 : /sbin/agetty ttyS0 9600 vt100-nav
2500 : /sbin/mingetty /dev/tty6
`, "\n")
	dTest = strings.Split(`26068 : /usr/bin/mongod -f /etc/mongod2.conf
25930 : /usr/bin/mongod -f /etc/mongod.conf
693 : /sbin/udevd -d
`, "\n")
	rsyslogTest = []string{"1129 : /sbin/rsyslogd"}

	nagiosTest = strings.Split(`2494 : /sbin/mingetty /dev/tty3
2496 : /sbin/mingetty /dev/tty4
12462 : /usr/sbin/nagios -d /etc/nagios/nagios.cfg
`, "\n")

	sshTest = strings.Split(`2812 : sshd: ec2-user [priv]
sshd: ec2-user@pts/0
`, "\n")

	javaTomcatTest   = []string{"1623 : /usr/bin/java -Djava.util.logging.config.file=/spatialkey/nexust/conf/logging.properties -Djava.util.logging.manager=org.apache.juli.ClassLoaderLogManager -Djava.endorsed.dirs=/spatialkey/nexust/endorsed -classpath /spatialkey/nexust/bin/bootstrap.jar:/spatialkey/nexust/bin/tomcat-juli.jar -Dcatalina.base=/spatialkey/nexust -Dcatalina.home=/spatialkey/nexust -Djava.io.tmpdir=/spatialkey/nexust/temp org.apache.catalina.startup.Bootstrap start"}
	javaNoTomcatTest = []string{"1423 : /etc/alternatives/java -Dcom.sun.akuma.Daemon=daemonized -Djava.awt.headless=true -DJENKINS_HOME=/var/lib/jenkins -jar /usr/lib/jenkins/jenkins.war --logfile=/var/log/jenkins/jenkins.log --webroot=/var/cache/jenkins/war --daemon --httpPort=8080 --ajp13Port=8009 --debug=5 --handlerCountMax=100 --handlerCountMaxIdle=20"}

	// This has embedded eol spaces, due to ridiculous null-to-space translation
	cent6Test = strings.Split(`1129 : /sbin/rsyslogd -i /var/run/syslogd.pid -c 5
2814 : sshd: ec2-user@pts/0
1780 : /usr/sbin/httpd
1781 : /usr/sbin/httpd
1782 : /usr/sbin/httpd
1783 : crond
1785 : /usr/sbin/httpd
1774 : /usr/sbin/httpd
1712 : /usr/libexec/postfix/master
1423 : /etc/alternatives/java -Dcom.sun.akuma.Daemon=daemonized -Djava.awt.headless=true -DJENKINS_HOME=/var/lib/jenkins -jar /usr/lib/jenkins/jenkins.war --logfile=/var/log/jenkins/jenkins.log --webroot=/var/cache/jenkins/war --daemon --httpPort=8080 --ajp13Port=8009 --debug=5 --handlerCountMax=100 --handlerCountMaxIdle=20
1776 : /usr/sbin/httpd
697 : /sbin/udevd -d
1772 : /usr/sbin/cronolog /spatialkey/tomcat/logs/mod_jk/%Y_%m_%d.log
1170 : dbus-daemon --system
1779 : /usr/sbin/httpd
1778 : /usr/sbin/httpd
1 : /sbin/init
1468 : /usr/sbin/sshd
1823 : /sbin/mingetty /dev/tty2
1751 : abrt-dump-oops -d /var/spool/abrt -rwx /var/log/messages
2815 : -bash
1623 : /usr/bin/java -Djava.util.logging.config.file=/spatialkey/nexust/conf/logging.properties -Djava.util.logging.manager=org.apache.juli.ClassLoaderLogManager -Djava.endorsed.dirs=/spatialkey/nexust/endorsed -classpath /spatialkey/nexust/bin/bootstrap.jar:/spatialkey/nexust/bin/tomcat-juli.jar -Dcatalina.base=/spatialkey/nexust -Dcatalina.home=/spatialkey/nexust -Djava.io.tmpdir=/spatialkey/nexust/temp org.apache.catalina.startup.Bootstrap start
1799 : /usr/sbin/atd
1230 : hald
1231 : hald-runner
718 : /sbin/udevd -d
1099 : auditd
1152 : irqbalance --pid=/var/run/irqbalance.pid
1490 : ntpd -u ntp:ntp -p /var/run/ntpd.pid -g
2812 : sshd: ec2-user [priv]
1831 : /sbin/mingetty /dev/tty6
1043 : /sbin/dhclient -1 -q -cf /etc/dhcp/dhclient-eth0.conf -lf /var/lib/dhclient/dhclient-eth0.leases -pf /var/run/dhclient-eth0.pid eth0
1829 : /sbin/mingetty /dev/tty5
1825 : /sbin/mingetty /dev/tty3
1827 : /sbin/mingetty /dev/tty4
1821 : /sbin/agetty /dev/hvc0 38400 vt100-nav
1820 : /sbin/mingetty /dev/tty1
1720 : qmgr -l -t fifo -u
1763 : /usr/sbin/httpd
1479 : xinetd -stayalive -pidfile /var/run/xinetd.pid
1741 : /usr/sbin/abrtd
376 : /sbin/udevd -d
`, "\n")
)

func TestNeedsRestartingMangler_Empty(t *testing.T) {
	v := needsRestartingMangler(emptyTest, emptyTest)
	if len(v) > 0 {
		t.Error("Expected empty array, got ", v)
	}
}

func TestNeedsRestartingMangler_NoSshd(t *testing.T) {
	v := needsRestartingMangler(sshTest, emptyTest)
	if len(v) > 0 {
		t.Error("Expected empty array, got ", v)
	}
}

func TestNeedsRestartingMangler_NoTomcat(t *testing.T) {
	v := needsRestartingMangler(javaNoTomcatTest, emptyTest)
	if len(v) > 0 {
		t.Error("Expected empty array, got ", v)
	}
}

func TestNeedsRestartingMangler_Tomcat(t *testing.T) {
	v := needsRestartingMangler(javaTomcatTest, emptyTest)
	if stringArrayEquality(v, []string{"tomcat"}) == false {
		t.Error("Expected [tomcat], got ", v)
	}
}

func TestNeedsRestartingMangler_D(t *testing.T) {
	v := needsRestartingMangler(dTest, emptyTest)
	if stringArrayEquality(v, []string{"mongod", "udevd"}) == false {
		t.Error("Expected [mongod udevd], got ", v)
	}
}

func TestNeedsRestartingMangler_Nagios(t *testing.T) {
	v := needsRestartingMangler(nagiosTest, emptyTest)
	if stringArrayEquality(v, []string{"nagios"}) == false {
		t.Error("Expected [nagios], got ", v)
	}
}

func TestNeedsRestartingMangler_Rsyslog(t *testing.T) {
	v := needsRestartingMangler(rsyslogTest, emptyTest)
	if stringArrayEquality(v, []string{"rsyslog"}) == false {
		t.Error("Expected [rsyslog], got ", v)
	}
}

func TestNeedsRestartingMangler_Exclude(t *testing.T) {
	v := needsRestartingMangler(dTest, []string{"mongod"})
	if stringArrayEquality(v, []string{"udevd"}) == false {
		t.Error("Expected [udevd] (not mongod!), got ", v)
	}
}

// We don't support the cent6 broken-ass syntax (yet?)
func TestNeedsRestartingMangler_Cent6(t *testing.T) {
	v := needsRestartingMangler(cent6Test, emptyTest)
	if stringArrayEquality(v, strings.Split("rsyslog tomcat hald abrtd xinetd httpd crond udevd sshd atd auditd ntpd", " ")) == false {
		t.Error("Expected specific 12-item array, got ", v)
	}
}

// Test arrays for makeList
var (
	emptyList   = []string{}
	simpleList  = strings.Split("hello,world,this,is,a,test", ",")
	complexList = []string{"hello world", "this is a test", "of some, cool features, of this function", "called makeList"}
)

func TestMakeList_Empty(t *testing.T) {
	v := makeList(emptyList)
	if len(v) > 0 {
		t.Error("Expected empty array, got ", v)
	}
}

func TestMakeList_Simple(t *testing.T) {
	v := makeList(simpleList)
	if len(v) != 6 {
		t.Error("Expected 6-item array, got ", v)
	}
}

func TestMakeList_Complex(t *testing.T) {
	v := makeList(complexList)
	if len(v) != 6 {
		t.Error("Expected 6-item array, got ", v)
	}
}

// We test 1000 strings of every size 8 to 99,
// and make sure they're the right size, and don't collide.
// Good enough for our use.
func TestRandString(t *testing.T) {
	for i := 8; i < 100; i++ {
		theStrings := make(map[string]bool)
		for c := 0; c < 1000; c++ {
			s := randString(i)
			if len(s) != i {
				t.Errorf("String '%s' should be %d long, but is %d!\n", s, i, len(s))
			}
			if _, ok := theStrings[s]; ok {
				t.Error("Um, random string collision! ", s)
			}
			theStrings[s] = true
		}
	}
}

// Helper function to determine the equality of two string arrays
func stringArrayEquality(X, Y []string) bool {

	// Obvious
	if len(X) != len(Y) {
		return false
	}

	// map all of Y's strings...
	s := make(map[string]int)
	for _, y := range Y {
		s[y]++
	}
	// ... and balk if X has anything else
	for _, x := range X {
		if s[x] > 0 {
			continue
		}
		return false
	}
	return true
}
