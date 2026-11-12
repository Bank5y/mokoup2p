#!/bin/sh
# Seam-3 smoke test: real server + relay + two real devices (mokoup2pd with
# the pinned easytier-core) in docker compose. Verifies:
#   1. the devices ping each other's virtual IP over a direct path;
#   2. UDP broadcast reaches the other device through the virtual network;
#   3. blocking the direct path switches to the relay without breaking a
#      TCP connection, and unblocking switches back to direct;
#   4. a network the server never registered cannot use the relay.
# Needs docker with compose v2 and /dev/net/tun; no root on the host.
#
# Usage: deploy/smoketest/run.sh        (KEEP=1 leaves the containers up)
set -eu

cd "$(dirname "$0")"
export COMPOSE_PROJECT_NAME=${COMPOSE_PROJECT_NAME:-mokoup2p-smoke}
export ADMIN_USERNAME=admin ADMIN_PASSWORD=smoke-password
export MOKOUP2P_RELAY_HOST=relay MOKOUP2P_RELAY_TOKEN=smoke-relay-token
compose() { docker compose -f ../docker-compose.yml -f compose.smoke.yml "$@"; }
on() { svc=$1; shift; compose exec -T "$svc" "$@"; }

failures=0
pass() { echo "PASS: $*"; }
fail() { echo "FAIL: $*"; failures=$((failures + 1)); }
step() { echo; echo "== $*"; }

cleanup() {
	if [ "${KEEP:-0}" = 1 ]; then
		echo "containers left running (project $COMPOSE_PROJECT_NAME)"
	else
		compose down -v --remove-orphans >/dev/null 2>&1 || true
	fi
}
trap cleanup EXIT

# wait_for SECONDS DESCRIPTION COMMAND...: retry COMMAND once a second.
wait_for() {
	secs=$1 what=$2; shift 2
	i=0
	while [ "$i" -lt "$secs" ]; do
		if "$@" >/dev/null 2>&1; then return 0; fi
		i=$((i + 1)); sleep 1
	done
	echo "timed out after ${secs}s waiting for: $what"
	return 1
}

# path SVC PEER_VIP: "direct", "relay" or "none", as easytier-cli sees it.
path() {
	on "$1" easytier-cli -o json peer list 2>/dev/null |
		jq -r --arg ip "$2" '[.[] | select(.ipv4 == $ip)][0].cost // "none"
			| if . == "p2p" then "direct" elif startswith("relay") then "relay" else "none" end'
}
path_is() { [ "$(path "$1" "$2")" = "$3" ]; }

step "build and start"
compose up -d --build --quiet-pull

step "devices log in, create and join a room"
for c in client1 client2; do
	on $c mokoup2p server http://server:8080
	wait_for 30 "$c login" sh -c "printf '%s\n' '$ADMIN_PASSWORD' | docker compose -f ../docker-compose.yml -f compose.smoke.yml exec -T $c mokoup2p login -u $ADMIN_USERNAME"
done
code=$(on client1 mokoup2p room create smoke | sed -n 's/^房间号：//p')
echo "room code: $code"
on client1 mokoup2p join "$code"
on client2 mokoup2p join "$code"
vip() { on "$1" mokoup2p status | sed -n 's/^本机虚拟 IP：\([0-9.]*\).*/\1/p'; }
vip1=$(vip client1) vip2=$(vip client2)
bcast=$(echo "$vip1" | sed 's/\.[0-9]*$/.255/')
echo "client1 $vip1, client2 $vip2, subnet broadcast $bcast"
wait_for 30 "relay whitelist" sh -c "docker compose -f ../docker-compose.yml -f compose.smoke.yml logs relay | grep -q 'relaying 1 room network'"

step "1. ping over a direct path"
if wait_for 60 "client1 reaches client2" on client1 ping -c 1 -W 1 "$vip2" &&
	on client2 ping -c 3 -W 1 "$vip1" >/dev/null; then
	pass "virtual IPs ping both ways"
else
	fail "virtual IPs do not ping"
fi
if wait_for 60 "direct path" path_is client1 "$vip2" direct; then
	pass "client1 -> client2 path is direct"
else
	fail "client1 -> client2 path is $(path client1 "$vip2"), want direct"
fi

step "2. UDP broadcast"
# The receiver logs each datagram's source address. Only a source in the
# virtual subnet proves EasyTier carried it: a plain 255.255.255.255 also
# crosses the Docker bridge (source 172.x), a false positive.
compose exec -d client2 sh -c 'socat -u UDP4-RECVFROM:9999,reuseaddr,fork SYSTEM:"echo \$SOCAT_PEERADDR >> /tmp/bcast.log"'
sleep 1
received_from() { on client2 sh -c 'cat /tmp/bcast.log 2>/dev/null' | grep -qx "$1"; }
send() { on client1 sh -c "echo hello | socat -u - UDP4-DATAGRAM:$1:9999,broadcast$2"; }

send "$bcast" ""
if wait_for 5 "subnet broadcast" received_from "$vip1"; then
	pass "subnet broadcast $bcast arrives from $vip1"
else
	fail "subnet broadcast $bcast not received from $vip1"
fi
on client2 sh -c ': > /tmp/bcast.log'
send 255.255.255.255 ",so-bindtodevice=mokoup2p0"
if wait_for 5 "limited broadcast" received_from "$vip1"; then
	pass "255.255.255.255 bound to mokoup2p0 arrives from $vip1"
else
	fail "255.255.255.255 bound to mokoup2p0 not received from $vip1"
fi
on client2 sh -c ': > /tmp/bcast.log'
# Unbound 255.255.255.255 follows the default route, not the TUN. On this
# internal network there is none, so sending fails ("Network unreachable");
# with a default route it would leave through eth0 instead.
if send 255.255.255.255 "" 2>/dev/null; then
	sleep 2
	echo "info: unbound 255.255.255.255 arrived from: $(on client2 sh -c 'cat /tmp/bcast.log 2>/dev/null' | tr '\n' ' ')(not via EasyTier)"
else
	echo "info: unbound 255.255.255.255 cannot be sent: it follows the default route, not mokoup2p0"
fi

step "3. block the direct path, then restore it"
eth2=$(compose exec -T client2 hostname -i | tr -d '\r' | awk '{print $1}')
echo "client2 bridge address: $eth2"
# A TCP stream over the virtual network that must survive both switches.
compose exec -d client2 sh -c "socat -u TCP-LISTEN:7000,bind=$vip2 OPEN:/tmp/tcp.log,creat,append"
sleep 1
compose exec -d client1 sh -c "i=0; while true; do echo \$i; i=\$((i+1)); sleep 0.2; done | socat -u - TCP:$vip2:7000"
compose exec -d client1 sh -c "ping -i 0.2 $vip2 > /tmp/ping.log 2>&1"
sleep 2
on client1 iptables -I INPUT -s "$eth2" -j DROP
on client1 iptables -I OUTPUT -d "$eth2" -j DROP
t0=$(date +%s)
if wait_for 90 "relay path" path_is client1 "$vip2" relay; then
	pass "with the direct path blocked, client1 -> client2 goes through the relay (after $(($(date +%s) - t0))s)"
else
	fail "with the direct path blocked, path is $(path client1 "$vip2"), want relay"
fi
# EasyTier needs a while to declare the dead direct link gone; traffic
# resumes over the relay some seconds after the route changes.
if wait_for 60 "ping over the relay" on client1 ping -c 1 -W 1 "$vip2"; then
	pass "ping works over the relay (traffic resumed $(($(date +%s) - t0))s after the block)"
else
	fail "ping does not work over the relay"
fi
tcp_lines() { on client2 sh -c 'wc -l < /tmp/tcp.log'; }
lines_before=$(tcp_lines)
tcp_grew() { [ "$(tcp_lines)" -gt "$lines_before" ]; }
if wait_for 60 "TCP stream over the relay" tcp_grew; then
	pass "the TCP stream keeps flowing over the relay ($lines_before -> $(tcp_lines) lines)"
else
	fail "the TCP stream stalled over the relay at $lines_before lines"
fi
if path_is client1 "$vip2" relay; then
	pass "still relayed while the direct path is blocked"
else
	fail "path is $(path client1 "$vip2") while the direct path is blocked, want relay"
fi

on client1 iptables -D INPUT -s "$eth2" -j DROP
on client1 iptables -D OUTPUT -d "$eth2" -j DROP
t0=$(date +%s)
if wait_for 180 "direct path again" path_is client1 "$vip2" direct; then
	pass "after unblocking, client1 -> client2 is direct again (after $(($(date +%s) - t0))s)"
else
	fail "after unblocking, path is $(path client1 "$vip2"), want direct"
fi
sleep 2
on client1 pkill -INT ping || true
if on client1 pgrep -f "TCP:$vip2:7000" >/dev/null; then
	pass "the TCP connection survived both switches ($(on client2 sh -c 'wc -l < /tmp/tcp.log') lines received)"
else
	fail "the TCP connection broke during the switches"
fi
if on client1 ping -c 3 -W 1 "$vip2" >/dev/null && path_is client1 "$vip2" direct; then
	pass "ping works over the restored direct path"
else
	fail "ping over the restored direct path"
fi
on client1 grep "packet loss" /tmp/ping.log | sed 's/^/info: ping every 0.2s across both switches: /' 

step "4. an unregistered network cannot use the relay"
sleep 5
if on stranger1 ping -c 3 -W 1 10.250.0.2 >/dev/null 2>&1; then
	fail "stranger1 reaches stranger2 through the relay"
else
	pass "stranger1 cannot reach stranger2"
fi
if [ "$(on stranger1 easytier-cli -o json peer list | jq '[.[] | select(.cost != "Local")] | length')" = 0 ]; then
	pass "stranger1 has no peers (not even the relay)"
else
	fail "stranger1 has peers: $(on stranger1 easytier-cli peer list)"
fi
if compose logs relay | grep -q "network mokoup2p-00000000deadbeef not in whitelist"; then
	pass "relay log: network mokoup2p-00000000deadbeef not in whitelist"
else
	fail "relay log does not show the stranger refused"
fi

step "5. leaving the room stops easytier-core and removes the TUN device"
on client1 mokoup2p leave
if on client1 ip link show mokoup2p0 >/dev/null 2>&1; then
	fail "mokoup2p0 still exists after leaving"
else
	pass "mokoup2p0 is gone after leaving"
fi
if on client1 pidof easytier-core >/dev/null; then
	fail "easytier-core still runs after leaving"
else
	pass "no easytier-core runs after leaving"
fi

step "6. a non-root daemon with only CAP_NET_ADMIN (as in the systemd unit) joins"
# setpriv reproduces User= + AmbientCapabilities=CAP_NET_ADMIN +
# CapabilityBoundingSet=CAP_NET_ADMIN; the root daemon above has left the
# room, so the TUN name and ports are free.
on client1 sh -c 'mkdir -p /tmp/u && chown 1000:1000 /tmp/u'
compose exec -d client1 setpriv --reuid=1000 --regid=1000 --clear-groups \
	--inh-caps=-all,+net_admin --ambient-caps=-all,+net_admin --bounding-set=-all,+net_admin \
	env HOME=/tmp/u MOKOUP2P_SOCKET=/tmp/u/daemon.sock MOKOUP2P_STATE_DIR=/tmp/u/state mokoup2pd
asuser() { on client1 setpriv --reuid=1000 --regid=1000 --clear-groups env HOME=/tmp/u MOKOUP2P_SOCKET=/tmp/u/daemon.sock "$@"; }
wait_for 10 "non-root daemon" asuser mokoup2p status
asuser mokoup2p server http://server:8080
printf '%s\n' "$ADMIN_PASSWORD" | asuser mokoup2p login -u "$ADMIN_USERNAME"
if asuser mokoup2p join "$code" && wait_for 30 "non-root device reaches client2" on client1 ping -c 1 -W 1 "$vip2"; then
	core=$(on client1 sh -c 'p=$(pidof easytier-core); echo "uid $(stat -c %u /proc/$p) $(grep CapEff /proc/$p/status | tr "\t" " ")"')
	pass "non-root daemon joined (easytier-core: $core), ping $vip2 ok"
else
	fail "non-root daemon with CAP_NET_ADMIN could not join and ping"
fi
asuser mokoup2p leave >/dev/null || true

echo
if [ "$failures" -eq 0 ]; then
	echo "smoke test passed"
else
	echo "smoke test: $failures failure(s)"
	exit 1
fi
