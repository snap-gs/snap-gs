#!/bin/bash

IFS=; set -euo pipefail; shopt -s nullglob

: ${AWS_METADATA_IDENTDOCURL:=http://169.254.169.254/latest/dynamic/instance-identity/document}
: ${SNAPGS_INSTALL_S3SYNCURL:=https://github.com/larrabee/s3sync/releases/download/2.34/s3sync_2.34_Linux_x86_64.tar.gz}
: ${SNAPGS_INSTALL_LOBBIES:=$(seq -s, $(lscpu --parse=CPU | grep -c '^[0-9]'))}
IFS=, read -r -a SNAPGS_INSTALL_DISABLE <<<${SNAPGS_INSTALL_DISABLE-}
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
			ua-reboot-cmds.service \
			ua-timer.timer \
			unattended-upgrades.service \
			update-notifier-download.timer \
			update-notifier-motd.timer \
				--now
	fi

	if ! (command -v go && command -v git && command -v steamcmd &&
				command -v jq && command -v xattr && command -v gcc && command -v aws && command -v tree) > /dev/null
	then
		sudo dpkg --add-architecture i386
		sudo apt update
		sudo debconf-set-selections <<<'steam steam/license note '
		sudo debconf-set-selections <<<'steam steam/question select I AGREE'
		sudo apt install --yes --no-install-recommends golang-go git steamcmd jq xattr build-essential awscli tree
	fi
	if ! (command -v s3sync) > /dev/null; then
		curl -sL $SNAPGS_INSTALL_S3SYNCURL |
			sudo tar --extract --gunzip --no-same-owner --directory=/usr/local/bin -- s3sync
	fi

	if ! id -u snap-gs > /dev/null 2>&1; then
		sudo useradd --user-group --create-home --home-dir /opt/snap-gs --shell /usr/sbin/nologin --uid 1001 snap-gs
	fi
	if ! id -u snap-gs-remote > /dev/null 2>&1; then
		sudo useradd --user-group --create-home --home-dir /opt/snap-gs-remote --shell /bin/bash --uid 1002 snap-gs-remote
	fi
	sudo -u snap-gs chmod 755 /opt/snap-gs
	sudo -u snap-gs mkdir -p /opt/snap-gs/SnapshotVR
	sudo -u snap-gs-remote chmod 755 /opt/snap-gs-remote
	sudo -u snap-gs-remote mkdir -p -m 700 /opt/snap-gs-remote/.ssh
	sudo -u snap-gs-remote touch -a /opt/snap-gs-remote/.ssh/authorized_keys
	sudo -u snap-gs-remote chmod 600 /opt/snap-gs-remote/.ssh/authorized_keys
	if [[ -e ~/.ssh/authorized_keys ]] && sudo -u snap-gs-remote test ! -s /opt/snap-gs-remote/.ssh/authorized_keys; then
		sudo -u snap-gs-remote tee /opt/snap-gs-remote/.ssh/authorized_keys > /dev/null < ~/.ssh/authorized_keys
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

	if ! [[ -d ~/snap-gs ]]; then
		git -C ~ clone git@github.com:snap-gs/snap-gs
	elif [[ $SNAPGS_INSTALL_ACCOUNT == 051813673067 ]] && git -C ~/snap-gs diff --quiet origin/HEAD; then
		git -C ~/snap-gs remote update --prune
		git -C ~/snap-gs reset --hard origin/HEAD
	fi
	cd ~/snap-gs; GOBIN=/tmp go install ./cmd/snap-gs; cd $OLDPWD
	sudo install --owner=snap-gs --group=snap-gs --mode=755 /tmp/snap-gs /opt/snap-gs/SnapshotVR/snap-gs.lock
	if [[ $SNAPGS_INSTALL_ACCOUNT == 051813673067 ]]; then
		gcc -nostartfiles -fpic -shared ~/snap-gs/hack/preload.c -o /tmp/preload.so -ldl -D_GNU_SOURCE
		sudo install --owner=snap-gs --group=snap-gs --mode=755 /tmp/preload.so /opt/snap-gs/SnapshotVR/snap-gs-preload.so.lock
		sudo install --owner=snap-gs --group=snap-gs --mode=755 ~/snap-gs/hack/sync.sh /opt/snap-gs/SnapshotVR/sync.sh.lock
		sudo mv /opt/snap-gs/SnapshotVR/snap-gs-preload.so{.lock,}
		sudo mv /opt/snap-gs/SnapshotVR/sync.sh{.lock,}
	fi
	sudo mv /opt/snap-gs/SnapshotVR/snap-gs{.lock,}

	ADDR=$(curl --silent $AWS_METADATA_IDENTDOCURL | jq -er .privateIp)
	ADDR1=$(curl --silent http://169.254.169.254/latest/meta-data/public-ipv4)
	ACCEL=$(curl --silent http://169.254.169.254/latest/meta-data/network/interfaces/macs/)
	ACCEL=$(curl --silent http://169.254.169.254/latest/meta-data/network/interfaces/macs/${ACCEL}subnet-id)
	ACCEL=$(aws --region us-west-2 globalaccelerator list-custom-routing-port-mappings-by-destination \
					--endpoint-id $ACCEL --destination-address $ADDR | jq -e -c || true)

	for i in ${SNAPGS_INSTALL_LOBBIES[@]}; do
		sudo systemctl stop gs.snap.lobby-SnapshotVR@$i.path || true
	done

	n=${#SNAPGS_INSTALL_LOBBIES[@]}
	for k in ${!SNAPGS_INSTALL_LOBBIES[@]}; do
		let 'i=SNAPGS_INSTALL_LOBBIES[k]'

		sudo -u snap-gs mkdir -p /opt/snap-gs/SnapshotVR/$i/{log,flag,stat,spec,sync}

		for d in ${!SNAPGS_INSTALL_DISABLE[@]}; do
			case ${SNAPGS_INSTALL_DISABLE[d]} in
				"discord") echo "127.$((d+1)).$((d+1)).1 discord.com discordapp.com discord.gg";;
				"stats")   echo "127.$((d+1)).$((d+1)).1 5xd0e4chtk.execute-api.us-east-1.amazonaws.com";;
			esac
		done | sudo -u snap-gs tee /opt/snap-gs/SnapshotVR/$i/hosts.lock > /dev/null
		sudo -u snap-gs tee -a /opt/snap-gs/SnapshotVR/$i/hosts.lock < /etc/hosts > /dev/null
		sudo mv /opt/snap-gs/SnapshotVR/$i/hosts{.lock,}

		unset ${!SNAPGS_LOBBY_@}
		SNAPGS_LOBBY_SESSION=
		SNAPGS_LOBBY_PASSWORD=
		if [[ -e /opt/snap-gs/SnapshotVR/$i/env ]]; then
			while read -r; do
				declare "${REPLY%%=*}=${REPLY##*=}"
			done < <(grep -E '^SNAPGS_LOBBY_' /opt/snap-gs/SnapshotVR/$i/env)
		fi
		if [[ ${1-} ]] ; then
			SNAPGS_LOBBY_SESSION=$1
		fi
		if [[ $SNAPGS_LOBBY_SESSION == */ ]]; then
			SNAPGS_LOBBY_SESSION="$SNAPGS_LOBBY_SESSION%s"
		elif [[ $k != 0 && $SNAPGS_LOBBY_SESSION == ${SNAPGS_LOBBY_SESSION//%s} ]]; then
			SNAPGS_LOBBY_SESSION="$SNAPGS_LOBBY_SESSION %s"
		fi
		printf -v SNAPGS_LOBBY_SESSION "$SNAPGS_LOBBY_SESSION" $i
		if [[ $# -gt 1 ]] ; then
			SNAPGS_LOBBY_PASSWORD=$2
		fi
		if [[ $SNAPGS_LOBBY_PASSWORD && $n == 1 || $k != 0 ]]; then
			SNAPGS_LOBBY_TIMEOUT=15m
		else
			SNAPGS_LOBBY_TIMEOUT=15h
		fi
		if [[ $SNAPGS_LOBBY_PASSWORD ]]; then
			SNAPGS_LOBBY_ADMINTIMEOUT=15h
		else
			SNAPGS_LOBBY_ADMINTIMEOUT=15m
		fi

		printf "%s=%s\n" \
				SNAPGS_LOBBY_SESSION "$SNAPGS_LOBBY_SESSION" \
				SNAPGS_LOBBY_PASSWORD "$SNAPGS_LOBBY_PASSWORD" \
				SNAPGS_LOBBY_TIMEOUT "$SNAPGS_LOBBY_TIMEOUT" \
				SNAPGS_LOBBY_ADMINTIMEOUT "$SNAPGS_LOBBY_ADMINTIMEOUT" \
		| sudo -u snap-gs tee /opt/snap-gs/SnapshotVR/$i/env.lock
		if [[ $SNAPGS_INSTALL_ACCOUNT == 051813673067 ]]; then
			PORT=$((27001+i))
			SNAPGS_LOBBY_LISTEN=$ADDR:$PORT,$ADDR1:$PORT,$(
				jq <<<"$ACCEL" -r --argjson i $((i % 2)) --argjson port $PORT '
					.DestinationPortMappings[]
					| select(.DestinationSocketAddress.Port==$port)
					| .AcceleratorSocketAddresses[$i]
					| "\(.IpAddress):\(.Port)"
				'
			)
			printf "%s=%s\n" \
					SNAPGS_LOBBY_LISTEN "$SNAPGS_LOBBY_LISTEN" \
			| sudo -u snap-gs tee -a /opt/snap-gs/SnapshotVR/$i/env.lock
			if [[ $SNAPGS_LOBBY_SESSION != test\ * ]]; then
				printf "%s=%s\n" \
						SNAPGS_SYNC_STATEBUCKET "public-snap-gs-lobby-$SNAPGS_INSTALL_REGION" \
						SNAPGS_SYNC_STATEREGION "$SNAPGS_INSTALL_REGION" \
						SNAPGS_SYNC_MATCHBUCKET "snap-gs-match-$SNAPGS_INSTALL_REGION" \
						SNAPGS_SYNC_MATCHREGION "$SNAPGS_INSTALL_REGION" \
						SNAPGS_SYNC_CLEANBUCKET "public-snap-gs-match-$SNAPGS_INSTALL_REGION" \
						SNAPGS_SYNC_CLEANREGION "$SNAPGS_INSTALL_REGION" \
						SNAPGS_SYNC_LOGBUCKET "snap-gs-lobby-$SNAPGS_INSTALL_REGION" \
						SNAPGS_SYNC_LOGREGION "$SNAPGS_INSTALL_REGION" \
				| sudo -u snap-gs tee -a /opt/snap-gs/SnapshotVR/$i/env.lock
			fi
		fi
		sudo mv /opt/snap-gs/SnapshotVR/$i/env{.lock,}

		sudo -u snap-gs rm -f /opt/snap-gs/SnapshotVR/$i/spec/{,force}{restart,stop,up,down}
		sudo -u snap-gs ln -s -f -T ../flag /opt/snap-gs/SnapshotVR/$i/spec/flag
		if [[ $n != 1 ]]; then
			if [[ $k == 0 ]]; then
				j=$((SNAPGS_INSTALL_LOBBIES[-1]))
			else
				j=$((SNAPGS_INSTALL_LOBBIES[k-1]))
			fi
			sudo -u snap-gs ln -s -f -T ../../$j/stat /opt/snap-gs/SnapshotVR/$i/spec/peer
		else
			sudo -u snap-gs rm -f /opt/snap-gs/SnapshotVR/$i/spec/peer
		fi

		for x in /opt/snap-gs/SnapshotVR/$i/flag/{forcerestart,forcestop,restart,stop,session,password}; do
			[[ -e $x ]] || sudo -u snap-gs touch $x
			[[ $(stat -c %a $x) == 666 ]] || sudo -u snap-gs chmod 666 $x
			[[ ${x##*/flag/force} == $x || -s $x ]] || sudo -u snap-gs truncate -s0 $x
		done
		if [[ $i == 1 && $SNAPGS_LOBBY_SESSION != test\ * ]] && [[ ! $SNAPGS_LOBBY_PASSWORD || $n != 1 ]]; then
			sudo -u snap-gs touch -a /opt/snap-gs/SnapshotVR/$i/flag/up
		else
			sudo -u snap-gs rm -f /opt/snap-gs/SnapshotVR/$i/flag/up
		fi

	done

	sudo ln -s -f ~/snap-gs/etc/sysctl.d/* /etc/sysctl.d
	sudo sysctl -q -p ~/snap-gs/etc/sysctl.d/*
	sudo systemctl link ~/snap-gs/etc/systemd/system/*
	sudo systemctl daemon-reload
	for i in ${SNAPGS_INSTALL_LOBBIES[@]}; do
		sudo systemctl enable gs.snap.lobby.sync-SnapshotVR@$i.path
		sudo systemctl restart gs.snap.lobby.sync-SnapshotVR@$i.path
		sudo systemctl enable gs.snap.lobby-SnapshotVR@$i.path
		sudo systemctl restart gs.snap.lobby-SnapshotVR@$i.path
		if [[ -e /opt/snap-gs/SnapshotVR/$i/stat/up ]]; then
			jq -jn "now|todateiso8601|tojson" |
				sudo -u snap-gs tee /opt/snap-gs/SnapshotVR/$i/spec/restart > /dev/null
		fi
	done

	echo DONE
}

main "$@"
