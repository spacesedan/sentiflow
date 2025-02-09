#!/bin/bash
sudo apt update -y
sudo apt install -y default-jre
wget https://downloads.apache.org/kafka/3.5.1/kafka_2.13-3.5.1.tgz
tar -xvzf kafka_2.13-3.5.1.tgz
cd kafka_2.13-3.5.1
nohup bin/zookeeper-server-start.sh config/zookeeper.properties >/dev/null 2>&1 &
nohup bin/kafka-server-start.sh config/server.properties >/dev/null 2>&1 &
