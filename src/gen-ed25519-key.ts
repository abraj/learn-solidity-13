import { keys } from '@libp2p/crypto';

function toHexString(uint8Array: Uint8Array): string {
  return Array.from(uint8Array)
    .map((byte) => byte.toString(16).padStart(2, '0'))
    .join('');
}

function toUint8Array(hexString: string): Uint8Array {
  if (hexString.length % 2 !== 0) {
    throw new Error('Invalid hex string');
  }
  // Create Uint8Array
  const byteArray = new Uint8Array(hexString.length / 2);

  for (let i = 0; i < hexString.length; i += 2) {
    byteArray[i / 2] = parseInt(hexString.substring(i, i + 2), 16);
  }

  return byteArray;
}

// ----------------------

export async function generatePrivateKey() {
  const privateKey = await keys.generateKeyPair('Ed25519');
  const privateKeyStr = toHexString(privateKey.raw);
  return privateKeyStr;
}

export function getPrivateKey(privateKeyHexStr: string) {
  const privateKeyRaw = toUint8Array(privateKeyHexStr);
  const privateKey = keys.privateKeyFromRaw(privateKeyRaw);
  return privateKey;
}

// ----------------------

// const privateKeyStr = await generatePrivateKey();
// console.log('privateKey:', privateKeyStr);

// const privateKey = getPrivateKey(privateKeyStr);
// console.log('privateKey:', privateKey);
