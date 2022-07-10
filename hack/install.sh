#!/bin/bash

IFS=; set -euo pipefail; shopt -s nullglob

: ${AWS_METADATA_IDENTDOCURL:=http://169.254.169.254/latest/dynamic/instance-identity/document}
: ${SNAPGS_INSTALL_S3SYNCURL:=https://github.com/larrabee/s3sync/releases/download/2.34/s3sync_2.34_Linux_x86_64.tar.gz}
: ${SNAPGS_INSTALL_LOBBIES:=$(seq -s, $(lscpu --parse=CPU | grep -c '^[0-9]'))}
IFS=, read -r -a SNAPGS_INSTALL_LOBBIES <<<$SNAPGS_INSTALL_LOBBIES

main () {
	if sudo systemctl is-enabled --quiet update-notifier-motd.timer; then
		sudo systemctl disable \
			apport-autoreport.timer \
			apt-daily-upgrade.timer \
			apt-daily.timer \
			dpkg-db-backup.timer \
			man-db.timer \
			motd-news.timer \
			ua-license-check.path \
			ua-reboot-cmds.service \
			ua-timer.timer \
			unattended-upgrades.service \
			update-notifier-download.timer \
			update-notifier-motd.timer \
				--now
	fi

	if ! (command -v go && command -v git && command -v steamcmd && command -v inotifywait &&
				command -v jq && command -v xattr && command -v gcc && command -v aws) > /dev/null
	then
		sudo dpkg --add-architecture i386
		sudo apt update
		sudo debconf-set-selections <<<'steam steam/license note '
		sudo debconf-set-selections <<<'steam steam/question select I AGREE'
		sudo apt install --yes --no-install-recommends golang-go git inotify-tools steamcmd jq xattr build-essential awscli
	fi
	if [[ ${SNAPGS_INSTALL_ACCOUNT-} ]]; then
		:
	elif SNAPGS_INSTALL_ACCOUNT=$(curl -s $AWS_METADATA_IDENTDOCURL | jq -er .accountId); then
		:
	else
		SNAPGS_INSTALL_REGION=
	fi
	if [[ ${SNAPGS_INSTALL_REGION-} ]]; then
		:
	elif SNAPGS_INSTALL_REGION=$(curl -s $AWS_METADATA_IDENTDOCURL | jq -er .region); then
		:
	else
		SNAPGS_INSTALL_REGION=
	fi
	if ! (command -v s3sync) > /dev/null; then
		curl -sL $SNAPGS_INSTALL_S3SYNCURL |
			sudo tar --extract --gunzip --no-same-owner --directory=/usr/local/bin -- s3sync
	fi
	if ! id -u snap-gs > /dev/null; then
		sudo useradd --user-group --create-home --home-dir /opt/snap-gs --shell /usr/sbin/nologin --uid 1001 snap-gs
	fi

	ACCEL=
	ADDR=$(curl -s $AWS_METADATA_IDENTDOCURL | jq -er .privateIp)
	ADDR1=$(curl -s http://169.254.169.254/latest/meta-data/public-ipv4)
	if [[ $SNAPGS_INSTALL_ACCOUNT == 051813673067 ]]; then
		ACCEL=$(curl --silent http://169.254.169.254/latest/meta-data/network/interfaces/macs/)
		ACCEL=$(curl --silent http://169.254.169.254/latest/meta-data/network/interfaces/macs/${ACCEL}subnet-id)
		ACCEL=$(aws --region us-west-2 globalaccelerator list-custom-routing-port-mappings-by-destination \
						--endpoint-id $ACCEL --destination-address $ADDR | jq -e -c || true)
	fi

	for k in ${!SNAPGS_INSTALL_LOBBIES[@]}; do
		let 'i=SNAPGS_INSTALL_LOBBIES[k]'

		sudo -u snap-gs mkdir -p /opt/snap-gs/SnapshotVR/$i/{stat,spec,sync}
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
		SNAPGS_LOBBY_SESSION=
		SNAPGS_LOBBY_PASSWORD=
		SNAPGS_LOBBY_ADMINTIMEOUT=
		if sudo -u snap-gs test -e /opt/snap-gs/SnapshotVR/$i/env; then
			while read -r; do
				declare "${REPLY%%=*}=${REPLY##*=}"
			done < <(sudo -u snap-gs grep -E '^SNAPGS_LOBBY_' /opt/snap-gs/SnapshotVR/$i/env)
		fi
		if [[ ${1-} ]] ; then
			SNAPGS_LOBBY_SESSION=$1
		fi
		if [[ $# -gt 1 ]] ; then
			SNAPGS_LOBBY_PASSWORD=$2
		fi
		if [[ $SNAPGS_LOBBY_SESSION == */ ]]; then
			SNAPGS_LOBBY_SESSION="$SNAPGS_LOBBY_SESSION%s"
		elif [[ $k -ne 0 && $SNAPGS_LOBBY_SESSION == ${SNAPGS_LOBBY_SESSION//%s} ]]; then
			SNAPGS_LOBBY_SESSION="$SNAPGS_LOBBY_SESSION %s"
		fi
		if [[ $SNAPGS_LOBBY_PASSWORD ]]; then
			SNAPGS_LOBBY_ADMINTIMEOUT=15h
		else
			SNAPGS_LOBBY_ADMINTIMEOUT=15m
		fi
		printf -v SNAPGS_LOBBY_SESSION "$SNAPGS_LOBBY_SESSION" $i
		printf "%s=%s\n" \
				SNAPGS_LOBBY_SESSION "$SNAPGS_LOBBY_SESSION" \
				SNAPGS_LOBBY_PASSWORD "$SNAPGS_LOBBY_PASSWORD" \
				SNAPGS_LOBBY_ADMINTIMEOUT "$SNAPGS_LOBBY_ADMINTIMEOUT" \
		| sudo -u snap-gs tee /opt/snap-gs/SnapshotVR/$i/env
		if [[ $SNAPGS_INSTALL_ACCOUNT == 051813673067 ]]; then
			PORT=$((27001+i))
			SNAPGS_LOBBY_LISTEN=$ADDR:$PORT
			SNAPGS_LOBBY_LISTEN1=$ADDR1:$PORT
			SNAPGS_LOBBY_LISTEN2=$(
				jq <<<"$ACCEL" -r --argjson i $((i % 2)) --argjson port $PORT '
					.DestinationPortMappings[]
					| select(.DestinationSocketAddress.Port==$port)
					| .AcceleratorSocketAddresses[$i]
					| "\(.IpAddress):\(.Port)"
				'
			)
			if [[ ! $SNAPGS_LOBBY_LISTEN2 ]]; then
				SNAPGS_LOBBY_LISTEN2=$ADDR1:$PORT
			fi
			printf "%s=%s\n" \
					SNAPGS_LOBBY_LOGSTATE true \
					SNAPGS_LOBBY_LOGMATCH true \
					SNAPGS_LOBBY_LOGCLEAN true \
					SNAPGS_LOBBY_LISTEN $SNAPGS_LOBBY_LISTEN \
					SNAPGS_LOBBY_LISTEN1 $SNAPGS_LOBBY_LISTEN1 \
					SNAPGS_LOBBY_LISTEN2 $SNAPGS_LOBBY_LISTEN2 \
					SNAPGS_SYNC_STATEBUCKET public-snap-gs-lobby-$SNAPGS_INSTALL_REGION \
					SNAPGS_SYNC_STATEREGION $SNAPGS_INSTALL_REGION \
					SNAPGS_SYNC_MATCHBUCKET snap-gs-match-$SNAPGS_INSTALL_REGION \
					SNAPGS_SYNC_MATCHREGION $SNAPGS_INSTALL_REGION \
					SNAPGS_SYNC_CLEANBUCKET public-snap-gs-match-$SNAPGS_INSTALL_REGION \
					SNAPGS_SYNC_CLEANREGION $SNAPGS_INSTALL_REGION \
					SNAPGS_SYNC_LOGBUCKET snap-gs-lobby-$SNAPGS_INSTALL_REGION \
					SNAPGS_SYNC_LOGREGION $SNAPGS_INSTALL_REGION \
			| sudo -u snap-gs tee -a /opt/snap-gs/SnapshotVR/$i/env
		fi

	done

	for repo in https://github.com/snap-gs/snap-gs; do
		if ! [[ -d ~/${repo##*/} ]]; then
			git -C ~ clone $repo
		elif git diff --quiet; then
			git -C ~/${repo##*/} remote update -p
			git -C ~/${repo##*/} reset --hard origin/HEAD
		fi
	done

	sudo ln -s -f ~/snap-gs/etc/sysctl.d/* /etc/sysctl.d
	sudo sysctl -q -p ~/snap-gs/etc/sysctl.d/*
	sudo systemctl link ~/snap-gs/etc/systemd/system/*
	sudo systemctl daemon-reload

	cd ~/snap-gs; go build -o /tmp/snap-gs ./cmd/snap-gs; cd $OLDPWD
	sudo install --owner=snap-gs --group=snap-gs --mode=755 /tmp/snap-gs /opt/snap-gs/SnapshotVR/snap-gs.lock
	if [[ $SNAPGS_INSTALL_ACCOUNT == 051813673067 ]]; then
		gcc -nostartfiles -fpic -shared ~/snap-gs/hack/preload.c -o /tmp/preload.so -ldl -D_GNU_SOURCE
		sudo install --owner=snap-gs --group=snap-gs --mode=755 /tmp/preload.so /opt/snap-gs/SnapshotVR/snap-gs-preload.so.lock
		sudo install --owner=snap-gs --group=snap-gs --mode=755 ~/snap-gs/hack/sync.sh /opt/snap-gs/SnapshotVR/sync.sh.lock
		sudo mv /opt/snap-gs/SnapshotVR/snap-gs-preload.so{.lock,}
		sudo mv /opt/snap-gs/SnapshotVR/sync.sh{.lock,}
	fi
	sudo mv /opt/snap-gs/SnapshotVR/snap-gs{.lock,}

	for i in ${SNAPGS_INSTALL_LOBBIES[@]}; do
		if [[ ${1-} != *-test ]] && [[ $SNAPGS_INSTALL_ACCOUNT == 051813673067 ]]; then
			sudo systemctl enable gs.snap.lobby.sync-SnapshotVR@$i.path
			sudo systemctl restart gs.snap.lobby.sync-SnapshotVR@$i.path
		fi
		sudo systemctl enable gs.snap.lobby.idle-SnapshotVR@$i.path
		sudo systemctl restart gs.snap.lobby.idle-SnapshotVR@$i.path
		sudo systemctl enable gs.snap.lobby-SnapshotVR@$i.path
		sudo systemctl restart gs.snap.lobby-SnapshotVR@$i.path
		if [[ ${1-} != *-test ]] && [[ $i == 1 || ${1-} == VRML* || ${1-} == VXL* ]]; then
			sudo systemctl enable gs.snap.lobby-SnapshotVR@$i.timer
			sudo systemctl restart gs.snap.lobby-SnapshotVR@$i.timer
		else
			sudo systemctl disable gs.snap.lobby-SnapshotVR@$i.timer
			sudo systemctl stop gs.snap.lobby-SnapshotVR@$i.timer
		fi
	done

	echo DONE
}

main "$@"
