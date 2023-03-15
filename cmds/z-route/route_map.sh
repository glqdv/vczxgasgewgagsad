#!/bin/sh /etc/rc.common

#Author: Fool

START=99
SERVICE_WRITE_PID=1
SERVICE_DAEMONIZE=1
PORT=1091
KCPEE="/usr/local/kcpee-scripts/Kcpee"
KCPEEMAP="/usr/bin/KcpeeMap"
CONFIG_DIR="/etc/kcpee-scripts"
LOGFILE="/etc/kcpee-scripts/proxy.log"

start_kcpee() {
	NAME="$(cat $CONFIG_DIR/user | grep name | awk -F = '{print $2}' | xargs )"
	PWD="$(cat $CONFIG_DIR/user | grep password | awk -F = '{print $2}' | xargs )"
	echo "Name: $NAME"
	echo "Pwd: $PWD"

	$KCPEE -Auth -name $NAME -pwd $PWD -dns -d;
	if [[  "$(ps | grep $KCPEEMAP | grep -v grep  )" == "" ]]; then
		$KCPEEMAP  -d;
	fi

}

start_redsocks() {
	echo $CONFIG_DIR/redsocks.conf
	redsocks2 -c  $CONFIG_DIR/redsocks.conf;
}


boot() {
    echo "Sleep 15s"
    sleep 15s
    start
}

start() {

    clean_init
    start_kcpee
    start_redsocks
    start_firewall
    /etc/init.d/firewall restart 2> /dev/null ;

}

kill_pro() {
   echo "stoping : $1"
   ps | grep $1 | grep -v KcpeeMap |grep -v grep | awk '{ print $1 } ' | xargs kill -9 2>/dev/null;
}



stop() {
   echo "#" > /etc/firewall.user;
   /etc/init.d/firewall restart 2> /dev/null;
   kill_pro Kcpee
   kill_pro redsocks2

}



clean_init() {
 stop
 echo ".... wait init wan .... 3 s"
 ifup wan ;
 sleep 3s;

}


start_firewall() {

REMOTE_IP=""
REMOTE_TXT="$CONFIG_DIR/remote.txt"
if [ -f $REMOTE_TXT ] ; then
    for l in $(cat $REMOTE_TXT) ; do
	if [[ $l != "" ]] ; then
		REMOTE_IP="$REMOTE_IP
iptables -t nat -A REDSOCKS -d $l -j RETURN"
		echo $l;
	fi
    done
fi


echo "============================================="
echo "==================  END ====================="

IFACE="$(ifconfig | awk '{print $1}' | grep wl| xargs)"
IP="$(ifconfig $IFACE | grep 192 | awk '{print $2}' | awk -F : '{print $2}')"

echo "Iface : $IFACE"
echo "MyIP:  $IP"

cat << EOF > /etc/firewall.user
iptables -t nat -F
iptables -t nat -N REDSOCKS

iptables -t nat -A PREROUTING -i $IFACE -p tcp -j REDSOCKS

# redirct dns
iptables -t nat -A PREROUTING -p udp --dport  53 -j REDIRECT --to-ports 60053

$REMOTE_IP

iptables -t nat -A REDSOCKS -d 0.0.0.0/8 -j RETURN
iptables -t nat -A REDSOCKS -d 10.0.0.0/8 -j RETURN
iptables -t nat -A REDSOCKS -d 127.0.0.0/8 -j RETURN
iptables -t nat -A REDSOCKS -d 169.254.0.0/16 -j RETURN
iptables -t nat -A REDSOCKS -d 172.16.0.0/12 -j RETURN
iptables -t nat -A REDSOCKS -d 224.0.0.0/4 -j RETURN
iptables -t nat -A REDSOCKS -d 240.0.0.0/4 -j RETURN


iptables -t nat -A REDSOCKS -p tcp -s $IP --dport $PORT -j RETURN
iptables -t nat -A REDSOCKS -d $IP -j RETURN
iptables -t nat -A REDSOCKS -p tcp -j REDIRECT --to-ports 1081
iptables -t nat -A PREROUTING -p tcp -j REDSOCKS

EOF

cat << EOF > $CONFIG_DIR/redsocks.conf
base {
	log_debug = on;
	log_info = on;
	redirector = iptables;
	daemon = on;
}

redsocks{
	bind = "0.0.0.0:1081";
	relay = "$IP:$PORT";
	type = socks5;
	autoproxy = 0;
	timeout = 12;
}

EOF


}