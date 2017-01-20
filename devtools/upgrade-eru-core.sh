#!/bin/bash


set -e
deploy_mode=${1-test}
if [ $deploy_mode == "test" ]
then
  ssh c1-eru-2 << EOF
  sudo yum --enablerepo=ricebook clean metadata
  sudo yum makecache
  sudo yum remove -y eru-core
  sudo yum install -y eru-core
  sudo systemctl daemon-reload
  sudo systemctl restart eru-core.service
EOF
elif [ $deploy_mode == "prod" ]
then
  ssh c2-eru-1 << EOF
  sudo yum --enablerepo=ricebook clean metadata
  sudo yum makecache
  sudo yum remove -y eru-core
  sudo yum install -y eru-core
  sudo systemctl daemon-reload
  sudo systemctl restart eru-core.service
EOF
else
  exit 127
fi
