#!/bin/bash

IFS=; set -euo pipefail

: ${SNAPGS_INSTALL_S3SYNCURL:=https://github.com/larrabee/s3sync/releases/download/2.34/s3sync_2.34_Linux_x86_64.tar.gz}
: ${SNAPGS_INSTALL_LOBBIES:=$(seq -s, $(lscpu --parse=CPU | grep -c '^[0-9]'))}
IFS=, read -r -a SNAPGS_INSTALL_LOBBIES <<<$SNAPGS_INSTALL_LOBBIES

main () {
	if ! (command -v go && command -v git && command -v steamcmd && command -v inotifywait) > /dev/null; then
		sudo dpkg --add-architecture i386
		sudo apt update
		sudo debconf-set-selections <<<'steam steam/license note '
		sudo debconf-set-selections <<<'steam steam/question select I AGREE'
		sudo apt install --yes --no-install-recommends golang-go git inotify-tools steamcmd
	fi
	if ! (command -v s3sync) > /dev/null; then
		curl -sL $SNAPGS_INSTALL_S3SYNCURL |
			sudo tar --extract --gunzip --no-same-owner --directory=/usr/local/bin -- s3sync
	fi
	if ! id -u snap-gs > /dev/null; then
		sudo useradd --user-group --create-home --home-dir /opt/snap-gs --shell /usr/sbin/nologin --uid 1001 snap-gs
	fi

	for k in ${!SNAPGS_INSTALL_LOBBIES[@]}; do
		let 'i=SNAPGS_INSTALL_LOBBIES[k]'

		sudo -u snap-gs mkdir -p /opt/snap-gs/SnapshotVR/$i/{stat,spec}
		sudo -u snap-gs ln -s -f -T ../../snap-gs /opt/snap-gs/SnapshotVR/$i/spec/restart
		if [[ ${#SNAPGS_INSTALL_LOBBIES[@]} -eq 1 ]] ; then
			sudo -u snap-gs rm -f /opt/snap-gs/SnapshotVR/$i/{self,peer}
			sudo -u snap-gs rm -f /opt/snap-gs/SnapshotVR/$i/spec/{up,down,stop}
		else
			sudo -u snap-gs ln -s -f -T stat /opt/snap-gs/SnapshotVR/$i/self
			sudo -u snap-gs ln -s -f -T ../peer/full /opt/snap-gs/SnapshotVR/$i/spec/up
			sudo -u snap-gs ln -s -f -T ../peer/idle /opt/snap-gs/SnapshotVR/$i/spec/down
			if [[ $k -eq 0 ]]; then
				sudo -u snap-gs ln -s -f -T ../peer/up /opt/snap-gs/SnapshotVR/$i/spec/stop
				sudo -u snap-gs ln -s -f -T ../$((SNAPGS_INSTALL_LOBBIES[-1]))/self /opt/snap-gs/SnapshotVR/$i/peer
			else
				sudo -u snap-gs ln -s -f -T /dev/null /opt/snap-gs/SnapshotVR/$i/spec/stop
				sudo -u snap-gs ln -s -f -T ../$((SNAPGS_INSTALL_LOBBIES[k-1]))/self /opt/snap-gs/SnapshotVR/$i/peer
			fi
		fi

		unset ${!SNAPGS_LOBBY_@}
		SNAPGS_LOBBY_ROOMNAME=
		SNAPGS_LOBBY_PASSWORD=
		SNAPGS_LOBBY_ADMINTIMEOUT=
		if sudo -u snap-gs test -e /opt/snap-gs/SnapshotVR/$i/env; then
			while read -r; do
				declare "${REPLY%%=*}=${REPLY##*=}"
			done < <(sudo -u snap-gs grep -E '^SNAPGS_LOBBY_' /opt/snap-gs/SnapshotVR/$i/env)
		fi
		if [[ ${1-} ]] ; then
			SNAPGS_LOBBY_ROOMNAME=$1
		fi
		if [[ $# -gt 1 ]] ; then
			SNAPGS_LOBBY_PASSWORD=$2
		fi
		if [[ $SNAPGS_LOBBY_ROOMNAME == */ ]]; then
			SNAPGS_LOBBY_ROOMNAME=$SNAPGS_LOBBY_ROOMNAME$i
		fi
		if [[ $SNAPGS_LOBBY_PASSWORD ]]; then
			SNAPGS_LOBBY_ADMINTIMEOUT=15h
		else
			SNAPGS_LOBBY_ADMINTIMEOUT=15m
		fi
		printf "SNAPGS_LOBBY_%s=%s\n" \
				ROOMNAME "$SNAPGS_LOBBY_ROOMNAME" PASSWORD "$SNAPGS_LOBBY_PASSWORD" ADMINTIMEOUT "$SNAPGS_LOBBY_ADMINTIMEOUT" |
			sudo -u snap-gs tee /opt/snap-gs/SnapshotVR/$i/env

	done

	if [[ -d ~/snap-gs ]]; then
		git -C ~/snap-gs remote update -p
		git -C ~/snap-gs reset --hard origin/main
	else
		git clone https://github.com/snap-gs/snap-gs ~/snap-gs
	fi

	cd ~/snap-gs
	go build -o /tmp/snap-gs ./cmd/snap-gs
	sudo cp /tmp/snap-gs /opt/snap-gs/SnapshotVR/snap-gs
	sudo ln -s -f ~/snap-gs/etc/sysctl.d/* /etc/sysctl.d
	sudo sysctl -q -p ~/snap-gs/etc/sysctl.d/*
	sudo systemctl link ~/snap-gs/etc/systemd/system/*
	sudo systemctl daemon-reload
	for i in ${SNAPGS_INSTALL_LOBBIES[@]}; do
		sudo systemctl enable --now gs.snap.lobby.idle-SnapshotVR@$i.path
		sudo systemctl enable --now gs.snap.lobby-SnapshotVR@$i.path
		if [[ $i == 1 || ${1-} == VRML* || ${1-} == VXL* ]]; then
			sudo systemctl enable --now gs.snap.lobby-SnapshotVR@$i.timer
		else
			sudo systemctl disable --now gs.snap.lobby-SnapshotVR@$i.timer
		fi
	done
}

main "$@"