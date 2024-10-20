import process from 'node:process';
import { createLibp2p } from 'libp2p';
import type { Libp2pOptions } from 'libp2p';
import { mdns } from '@libp2p/mdns';
// import { tcp } from '@libp2p/tcp';
import { webSockets } from '@libp2p/websockets';
import { noise } from '@chainsafe/libp2p-noise';
import { yamux } from '@chainsafe/libp2p-yamux';
import { bootstrap } from '@libp2p/bootstrap';
import { kadDHT, removePrivateAddressesMapper } from '@libp2p/kad-dht';
import { identify, identifyPush } from '@libp2p/identify';
import { autoNAT } from '@libp2p/autonat';
import { uPnPNAT } from '@libp2p/upnp-nat';
// import { gossipsub } from '@chainsafe/libp2p-gossipsub';
import { peerIdFromString } from '@libp2p/peer-id';
import { multiaddr } from '@multiformats/multiaddr';
// import { ping } from '@libp2p/ping';
import { getPrivateKey } from './gen-ed25519-key.ts';

const main = async () => {
  const privateKeyStr =
    'b3e86e9eda6d665987a1e3eacf06d2bd28f37dda849e23ccf93be25562819d16a59cc2f01fe31c7bbf5a569d3bf1629ba8d9bf569647429e6b3e1a3ad3c25239';
  const privateKey = getPrivateKey(privateKeyStr);

  // Known peers addresses
  const bootstrapMultiaddrs: string[] = [
    // '/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb',
    // '/ip4/127.0.0.1/tcp/51600/ws/p2p/12D3KooWMEKUnhFqDEe75CAszgdXEgvuaSsWeVzSKGm8ZcnGW5u7',
    // '/ip4/172.232.108.85/tcp/8000/p2p/12D3KooWFYh5vhYERz5aC3vFUkyzktKNZ4WubYxPiFbFSFzqct4p',
    '/ip4/172.232.108.85/tcp/8000/ws/p2p/12D3KooWFYh5vhYERz5aC3vFUkyzktKNZ4WubYxPiFbFSFzqct4p',
  ];

  const node = await createLibp2p({
    privateKey,

    // peerRouters: ..
    // contentRouters: ..

    // libp2p nodes are started by default, pass false to override this
    start: false,

    // add a listen address (localhost) to accept TCP connections on a random port
    addresses: {
      listen: [
        // '/ip4/127.0.0.1/tcp/0/ws',
        // '/ip4/127.0.0.1/tcp/0',
        // '/ip4/172.235.29.4/tcp/8000',
        '/ip4/0.0.0.0/tcp/8000/ws',
      ],
    },

    transports: [
      // tcp(),
      webSockets(),
    ],
    connectionEncrypters: [noise()],
    streamMuxers: [yamux()],

    // discovering peers via a list of known peers (seed nodes)
    peerDiscovery: [
      mdns({
        interval: 1000, // interval to search for peers in ms
      }),
      bootstrap({
        list: bootstrapMultiaddrs, // provide array of multiaddrs
      }),
    ],

    services: {
      identify: identify(),
      identifyPush: identifyPush(),
      // DHT Configuration for distributed peer discovery and routing
      dht: kadDHT({
        // protocol: '/ipfs/lan/kad/1.0.0',
        // peerInfoMapper: removePublicAddressesMapper,
        protocol: '/ipfs/kad/1.0.0',
        peerInfoMapper: removePrivateAddressesMapper,
        clientMode: false, // Set to false so each node participates in the DHT and stores info
        // kBucketSize: 20,
      }),
      autoNAT: autoNAT(),
      // uPnPNAT: uPnPNAT(),
      // pubsub: gossipsub({
      //   emitSelf: false, // whether the node should emit to self on publish
      //   globalSignaturePolicy: SignaturePolicy.StrictSign, // message signing policy
      // }),
      // ping: ping({
      //   protocolPrefix: 'ipfs', // default
      // }),
    },
    // connectionManager: {
    //   maxConnections: 50, // Limit the number of simultaneous connections
    // },
  } as Libp2pOptions);

  node.addEventListener('peer:discovery', (evt) => {
    console.log('Discovered %s', evt.detail.id.toString()); // Log discovered peer
  });

  node.addEventListener('peer:connect', (evt) => {
    console.log('Connected to %s', evt.detail.toString()); // Log connected peer
  });

  node.addEventListener('peer:disconnect', (evt) => {
    console.log('Disconnected from %s', evt.detail.toString()); // Log disconnected peer
  });

  // start libp2p
  await node.start();
  console.log('libp2p has started');

  // print out listening addresses
  console.log('listening on addresses:');
  // console.log('>>', node.peerId.toString());
  const listenAddrs = node.getMultiaddrs();
  if (listenAddrs.length) {
    listenAddrs.forEach((addr) => {
      console.log('', addr.toString());
    });
  } else {
    console.log('', listenAddrs);
  }

  // // ping peer if received multiaddr
  // if (process.argv.length >= 3) {
  //   const ma = multiaddr(process.argv[2]);
  //   console.log(`pinging remote peer at ${process.argv[2]}`);
  //   const latency = await node.services.ping.ping(ma);
  //   console.log(`pinged ${process.argv[2]} in ${latency}ms`);
  // } else {
  //   console.log('no remote peer address given, skipping ping');
  // }

  setInterval(async () => {
    console.log('---------------');

    const peers = await node.peerStore.all();
    const knownPeers = peers.map((p) => p.id.toString());
    console.log('Known peers:', knownPeers);

    // const connections = node.connectionManager.getConnections();
    const connections = node.getConnections();
    const connectedPeers = connections.map((connection) =>
      connection.remotePeer.toString()
    );
    console.log('Connected peers:', connectedPeers);

    // const peerId = peerIdFromString(
    //   '12D3KooWEUfKNh9uzgSwEQx7aTVkoXAhhv6X2bxbfBVY22b4qnkd',
    // );
    // console.log('peerId:', peerId);
    // const peerInfo = await node.peerRouting.findPeer(peerId /*, { signal }*/);
    // console.info('peerInfo:', peerInfo); // peer id, multiaddrs
  }, 8000);

  // setTimeout(() => {
  //   console.log('DIAL..');
  //   const ma1 = multiaddr(
  //     '/ip4/172.232.108.85/tcp/8000/ws/p2p/12D3KooWFYh5vhYERz5aC3vFUkyzktKNZ4WubYxPiFbFSFzqct4p',
  //   );
  //   node.dial(ma1);
  // }, 11000);

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
