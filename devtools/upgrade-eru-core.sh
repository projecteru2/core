#!/bin/sh

ssh liuyifu@c2-eru-1.ricebook.link -t 'sudo yum --enablerepo=ricebook clean metadata'
ssh liuyifu@c2-eru-1.ricebook.link -t 'sudo yum reinstall -y eru-core'
ssh liuyifu@c2-eru-1.ricebook.link -t 'sudo systemctl daemon-reload'
ssh liuyifu@c2-eru-1.ricebook.link -t 'sudo systemctl restart eru-core.service'
ssh liuyifu@c2-eru-1.ricebook.link -t 'sudo git --work-tree=/opt/citadel --git-dir=/opt/citadel/.git pull'
ssh liuyifu@c2-eru-1.ricebook.link -t 'sudo systemctl restart citadel'
