#!/bin/bash

log_stderr()
{
    echo "[`date`] $@" >&2
}

rpc_port=$1
if [[ "x$rpc_port" == "x" ]]; then
    echo "need rpc_port as paramter!"
    exit 1
fi

netstat -nltu | grep ":$rpc_port " > /dev/null

# 检查 netstat 命令的退出状态
if [ $? -eq 0 ]; then
#  log_stderr "端口($rpc_port)已被占用，退出代码 0"
  exit 0
else
#  log_stderr "端口($rpc_port)未被占用，退出代码 1"
  exit 1
fi