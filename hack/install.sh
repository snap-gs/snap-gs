#!/bin/bash

IFS=; set -euo pipefail

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
	if ! id -u snap-gs > /dev/null; then
		sudo useradd --user-group --create-home --home-dir /opt/snap-gs --shell /usr/sbin/nologin --uid 1001 snap-gs
	fi

	for k in ${!SNAPGS_INSTALL_LOBBIES[@]}; do
		let 'i=SNAPGS_INSTALL_LOBBIES[k]'

		sudo -u snap-gs mkdir -p /opt/snap-gs/SnapshotVR/$i/{stat,spec}
		sudo -u snap-gs ln -s -f -T peer/full /opt/snap-gs/SnapshotVR/$i/spec/up
		sudo -u snap-gs ln -s -f -T peer/idle /opt/snap-gs/SnapshotVR/$i/spec/down
		sudo -u snap-gs ln -s -f -T ../../snap-gs /opt/snap-gs/SnapshotVR/$i/spec/restart
		case $k in
		0)
			sudo -u snap-gs ln -s -f -T peer/up /opt/snap-gs/SnapshotVR/$i/spec/stop
			if [[ ${#SNAPGS_INSTALL_LOBBIES[@]} -eq 1 ]] ; then
				sudo -u snap-gs rm -f /opt/snap-gs/SnapshotVR/$i/spec/peer
			else
				sudo -u snap-gs ln -s -f -T ../../$((SNAPGS_INSTALL_LOBBIES[-1]))/stat /opt/snap-gs/SnapshotVR/$i/spec/peer
			fi
			;;
		*)
			sudo -u snap-gs ln -s -f -T /dev/null /opt/snap-gs/SnapshotVR/$i/spec/stop
			if [[ ${#SNAPGS_INSTALL_LOBBIES[@]} -gt 1 ]] ; then
				sudo -u snap-gs ln -s -f -T ../../$((SNAPGS_INSTALL_LOBBIES[k-1]))/stat /opt/snap-gs/SnapshotVR/$i/spec/peer
			fi
			;;
		esac

		unset ${!SNAPGS_LOBBY_@}
		SNAPGS_LOBBY_ROOMNAME=
		SNAPGS_LOBBY_PASSWORD=
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
		printf "SNAPGS_LOBBY_%s=%s\n" ROOMNAME "$SNAPGS_LOBBY_ROOMNAME" PASSWORD "$SNAPGS_LOBBY_PASSWORD" |
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
		case $i in
		1) sudo systemctl enable --now gs.snap.lobby-SnapshotVR@$i.timer ;;
		esac
	done
}

main "$@"
