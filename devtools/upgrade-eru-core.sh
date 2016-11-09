#!/bin/bash

if [ $1 == "test" ]; then
  rsync -a /Users/timfeirg/gocode/src/gitlab.ricebook.net/platform/core sa-ricebook:~
  ssh sa-ricebook -t 'sudo rm -rf /root/.go/src/gitlab.ricebook.net/platform/core'
  ssh sa-ricebook -t 'sudo mv ~/core /root/.go/src/gitlab.ricebook.net/platform/core'
  # ssh zzz1 -t 'sudo mv eru-core /usr/bin/eru-core'
  # ssh zzz1 -t 'sudo systemctl restart eru-core.service'
else
  ssh liuyifu@c2-eru-1.ricebook.link -t 'sudo yum --enablerepo=ricebook clean metadata'
  ssh liuyifu@c2-eru-1.ricebook.link -t 'sudo yum reinstall -y eru-core'
  ssh liuyifu@c2-eru-1.ricebook.link -t 'sudo systemctl daemon-reload'
  ssh liuyifu@c2-eru-1.ricebook.link -t 'sudo systemctl restart eru-core.service'
  ssh liuyifu@c2-eru-1.ricebook.link -t 'sudo git --work-tree=/opt/citadel --git-dir=/opt/citadel/.git pull'
  ssh liuyifu@c2-eru-1.ricebook.link -t 'sudo systemctl restart citadel'
fi
