#!/bin/bash


set -e
deploy_mode=${1-test}
if [ $deploy_mode == "test" ]
then
  ssh c1-eru-1.ricebook.link -t 'sudo yum --enablerepo=ricebook clean metadata'
  ssh c1-eru-1.ricebook.link -t 'sudo yum makecache'
  ssh c1-eru-1.ricebook.link -t 'sudo yum install -y eru-core'
  ssh c1-eru-1.ricebook.link -t 'sudo systemctl daemon-reload'
  ssh c1-eru-1.ricebook.link -t 'sudo systemctl restart eru-core.service'
elif [ $deploy_mode == "prod" ]
then
  ssh c2-eru-1.ricebook.link -t 'sudo yum --enablerepo=ricebook clean metadata'
  ssh c2-eru-1.ricebook.link -t 'sudo yum makecache'
  ssh c2-eru-1.ricebook.link -t 'sudo yum install -y eru-core'
  ssh c2-eru-1.ricebook.link -t 'sudo systemctl daemon-reload'
  ssh c2-eru-1.ricebook.link -t 'sudo systemctl restart eru-core.service'
else
  exit 127
fi
