#!/bin/bash
set +e
# set +x

echo "Starting Schellyhook Sample API..."
schellyhook-sample \
    --listen-ip=$LISTEN_IP \
    --listen-port=$LISTEN_PORT \
    --log-level=$LOG_LEVEL \
    --pre-post-timeout=$PRE_POST_TIMEOUT_SECONDS \
    --pre-backup-command=$PRE_BACKUP_COMMAND \
    --post-backup-command=$POST_BACKUP_COMMAND

