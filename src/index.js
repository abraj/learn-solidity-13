import process from 'node:process';
import { createLibp2p } from 'libp2p';
import { tcp } from '@libp2p/tcp';
import { noise } from '@chainsafe/libp2p-noise';
import { mplex } from '@libp2p/mplex';
import { multiaddr } from 'multiaddr';
import { ping } from '@libp2p/ping';

const main = async () => {
  const node = await createLibp2p({
    addresses: {
      // add a listen address (localhost) to accept TCP connections on a random port
      listen: ['/ip4/127.0.0.1/tcp/0'],
    },
    transports: [tcp()],
    connectionEncrypters: [noise()],
    streamMuxers: [mplex()],
    services: {
      ping: ping({
        protocolPrefix: 'ipfs', // default
      }),
    },
  });

  // start libp2p
  await node.start();
  console.log('libp2p has started');

  // print out listening addresses
  console.log('listening on addresses:');
  // node.peerId.toString()
  node.getMultiaddrs().forEach((addr) => {
    console.log('', addr.toString());
  });

  // ping peer if received multiaddr
  if (process.argv.length >= 3) {
    const ma = multiaddr(process.argv[2]);
    console.log(`pinging remote peer at ${process.argv[2]}`);
    const latency = await node.services.ping.ping(ma);
    console.log(`pinged ${process.argv[2]} in ${latency}ms`);
  } else {
    console.log('no remote peer address given, skipping ping');
  }

  const stop = async () => {
    // stop libp2p
    await node.stop();
    console.log('libp2p has stopped');
    process.exit(0);
  };

  process.on('SIGTERM', stop);
  process.on('SIGINT', stop);
};

main().then().catch(console.error);

// TODO:
// Fix @libp2p/mplex deprecation
// Fix multiaddr deprecation --> @multiformats/multiaddr
// Add @libp2p/mplex to CONFIGURATION
// fix config: connectionEncryption --> connectionEncrypters
// fix typo: master/doc/GETTING_STARTED.md -> main/doc/GETTING_STARTED.md
