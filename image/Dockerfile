FROM hyperledger/fabric-baseos:x86_64-0.4.5
COPY docker/bin/kepler /usr/local/bin/
RUN mkdir -p /opt/gopath/src/github.com/vntchain/kepler
ADD config.tar.bz2 /opt/gopath/src/github.com/vntchain/kepler

ENV KEPLER_CFG_PATH /opt/gopath/src/github.com/vntchain/kepler/config 

CMD ["kepler", "node", "start"]