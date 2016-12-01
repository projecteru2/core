#!/bin/bash

if [ $1 == "test" ]; then
  ssh c1-eru-1.ricebook.link -t 'sudo yum --enablerepo=ricebook clean metadata'
  ssh c1-eru-1.ricebook.link -t 'sudo yum reinstall -y eru-core'
  ssh c1-eru-1.ricebook.link -t 'sudo systemctl daemon-reload'
  ssh c1-eru-1.ricebook.link -t 'sudo systemctl restart eru-core.service'
  ssh c1-eru-1.ricebook.link -t 'sudo git --work-tree=/opt/citadel --git-dir=/opt/citadel/.git fetch --all --prune'
  ssh c1-eru-1.ricebook.link -t 'sudo git --work-tree=/opt/citadel --git-dir=/opt/citadel/.git reset --hard origin/feature/celery'
  ssh c1-eru-1.ricebook.link -t 'sudo systemctl restart citadel'
  ssh c1-eru-1.ricebook.link -t 'sudo systemctl restart citadel-worker'
else
  ssh c2-eru-1.ricebook.link -t 'sudo yum --enablerepo=ricebook clean metadata'
  ssh c2-eru-1.ricebook.link -t 'sudo yum reinstall -y eru-core'
  ssh c2-eru-1.ricebook.link -t 'sudo systemctl daemon-reload'
  ssh c2-eru-1.ricebook.link -t 'sudo systemctl restart eru-core.service'
  ssh c2-eru-1.ricebook.link -t 'sudo git --work-tree=/opt/citadel --git-dir=/opt/citadel/.git pull'
  ssh c2-eru-1.ricebook.link -t 'sudo systemctl restart citadel'
fi
