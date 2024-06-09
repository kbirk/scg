#!/bin/bash

mkdir -p include

# install acutest
wget -O ./include/acutest.h https://raw.githubusercontent.com/mity/acutest/master/include/acutest.h

# install asio standlone
wget -O asio.tar.gz https://sourceforge.net/projects/asio/files/asio/1.30.2%20%28Stable%29/asio-1.30.2.tar.gz && \
	tar -xvzf asio.tar.gz && \
	cp -r ./asio-*/include/asio ./include/ && \
	cp -r  ./asio-*/include/asio.hpp ./include/ && \
	rm -rf asio-* && \
	rm asio.tar.gz

# install nlohman.json
mkdir -p include/nlohmann
wget -O ./include/nlohmann/json.hpp https://github.com/nlohmann/json/releases/download/v3.11.3/json.hpp

#install websocketpp
wget -O websocketpp.tar.gz https://github.com/zaphoyd/websocketpp/archive/refs/tags/0.8.2.tar.gz && \
	tar -xvzf websocketpp.tar.gz && \
	cp -r websocketpp-*/websocketpp ./include/ && \
	rm -rf websocketpp-* && \
	rm websocketpp.tar.gz
