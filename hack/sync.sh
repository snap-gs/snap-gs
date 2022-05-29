#!/bin/bash

IFS=; set -euo pipefail; shopt -s nullglob

main () {

	[[ $1 && $2 && -e $1 && -e $2 ]] || exit 1
	[[ $SNAPGS_SYNC_LOGBUCKET && $SNAPGS_SYNC_LOGREGION ]] || exit 1
	[[ $SNAPGS_SYNC_MATCHBUCKET && $SNAPGS_SYNC_MATCHREGION ]] || exit 1
	[[ $SNAPGS_SYNC_CLEANBUCKET && $SNAPGS_SYNC_CLEANREGION ]] || exit 1
	[[ $SNAPGS_SYNC_STATEBUCKET && $SNAPGS_SYNC_STATEREGION ]] || exit 1

	cd $1; s1=(*-lobby.log.gz); s2=(*-match.json.gz); s3=(*-clean.json.gz); s4=([s]tate.json.gz); cd $OLDPWD

	for ((i=0; i!=${#s1[@]}; i++)); do
		p=${s1[i]%.gz}; p=${p//[_Z]}; p=${p//[-T]/\/}
		lobby=$(xattr -p user.s3sync.meta $1/${s1[i]} | jq -er .metadata.lobby)
		mkdir -p $2/$SNAPGS_SYNC_LOGBUCKET/$lobby/${p%/*}
		mv $1/${s1[i]} $2/$SNAPGS_SYNC_LOGBUCKET/$lobby/$p
	done && if [[ -e $2/$SNAPGS_SYNC_LOGBUCKET/ ]]; then
		s3sync --s3-cache-control max-age=86400 --tr $SNAPGS_SYNC_LOGREGION \
			fs://$2/$SNAPGS_SYNC_LOGBUCKET s3://$SNAPGS_SYNC_LOGBUCKET
		find $2/$SNAPGS_SYNC_LOGBUCKET -type f -printf "%P\n" >> $2/$SNAPGS_SYNC_LOGBUCKET.log
		rm -rf $2/$SNAPGS_SYNC_LOGBUCKET
	fi &

	for ((i=0; i!=${#s2[@]}; i++)); do
		p=${s2[i]%.gz}; p=${p//[_Z]}; p=${p//[-T]/\/}
		lobby=$(xattr -p user.s3sync.meta $1/${s2[i]} | jq -er .metadata.lobby)
		mkdir -p $2/$SNAPGS_SYNC_MATCHBUCKET/$lobby/${p%/*}
		mv $1/${s2[i]} $2/$SNAPGS_SYNC_MATCHBUCKET/$lobby/$p
	done && if [[ -e $2/$SNAPGS_SYNC_MATCHBUCKET/ ]]; then
		s3sync --s3-cache-control max-age=300 --tr $SNAPGS_SYNC_MATCHREGION \
			fs://$2/$SNAPGS_SYNC_MATCHBUCKET s3://$SNAPGS_SYNC_MATCHBUCKET
		find $2/$SNAPGS_SYNC_MATCHBUCKET -type f -printf "%P\n" >> $2/$SNAPGS_SYNC_MATCHBUCKET.log
		rm -rf $2/$SNAPGS_SYNC_MATCHBUCKET
	fi &

	for ((i=0; i!=${#s3[@]}; i++)); do
		p=${s3[i]/%clean.json.gz/match.json}; p=${p//[_Z]}; p=${p//[-T]/\/}
		lobby=$(xattr -p user.s3sync.meta $1/${s3[i]} | jq -er .metadata.lobby)
		mkdir -p $2/$SNAPGS_SYNC_CLEANBUCKET/$lobby/${p%/*}
		mv $1/${s3[i]} $2/$SNAPGS_SYNC_CLEANBUCKET/$lobby/$p
	done && if [[ -e $2/$SNAPGS_SYNC_CLEANBUCKET/ ]]; then
		s3sync --s3-cache-control max-age=300 --tr $SNAPGS_SYNC_CLEANREGION \
			fs://$2/$SNAPGS_SYNC_CLEANBUCKET s3://$SNAPGS_SYNC_CLEANBUCKET
		find $2/$SNAPGS_SYNC_CLEANBUCKET -type f -printf "%P\n" >> $2/$SNAPGS_SYNC_CLEANBUCKET.log
		rm -rf $2/$SNAPGS_SYNC_CLEANBUCKET
	fi &

	for ((i=0; i!=${#s4[@]}; i++)); do
		p=${s4[i]%.gz}
		lobby=$(xattr -p user.s3sync.meta $1/${s4[i]} | jq -er .metadata.lobby)
		mkdir -p $2/$SNAPGS_SYNC_STATEBUCKET/$lobby
		mv $1/${s4[i]} $2/$SNAPGS_SYNC_STATEBUCKET/$lobby/$p
	done && if [[ -e $2/$SNAPGS_SYNC_STATEBUCKET/ ]]; then
		s3sync --s3-cache-control no-cache --tr $SNAPGS_SYNC_STATEREGION \
			fs://$2/$SNAPGS_SYNC_STATEBUCKET s3://$SNAPGS_SYNC_STATEBUCKET
		find $2/$SNAPGS_SYNC_STATEBUCKET -type f -printf "%P\n" >> $2/$SNAPGS_SYNC_STATEBUCKET.log
		rm -rf $2/$SNAPGS_SYNC_STATEBUCKET
	fi &

	wait

	rc=0
	for bucket in $2/*/; do
		case $bucket in
		$2/$SNAPGS_SYNC_LOGBUCKET/) 	rc=$(( rc | 1<<1 ));;
		$2/$SNAPGS_SYNC_MATCHBUCKET/) rc=$(( rc | 1<<2 ));;
		$2/$SNAPGS_SYNC_CLEANBUCKET/) rc=$(( rc | 1<<3 ));;
		esac
	done

	exit $rc
}

main "$@"
