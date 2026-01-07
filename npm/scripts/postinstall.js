#!/usr/bin/env node

const https = require('https');
const fs = require('fs');
const path = require('path');

const REPO = 'ArangoGutierrez/k8s-gpu-mcp-server';
const BINARY_NAME = 'k8s-gpu-mcp-server';

// Map Node.js platform/arch to Go build targets
const PLATFORM_MAP = {
  'darwin-x64': 'darwin-amd64',
  'darwin-arm64': 'darwin-arm64',
  'linux-x64': 'linux-amd64',
  'linux-arm64': 'linux-arm64',
  'win32-x64': 'windows-amd64',
};

function getPlatformKey() {
  const platform = process.platform;
  const arch = process.arch;
  return `${platform}-${arch}`;
}

function getBinaryName(platformKey) {
  const goPlatform = PLATFORM_MAP[platformKey];
  if (!goPlatform) {
    throw new Error(`Unsupported platform: ${platformKey}`);
  }

  const ext = process.platform === 'win32' ? '.exe' : '';
  return `${BINARY_NAME}-${goPlatform}${ext}`;
}

function getPackageVersion() {
  const packageJson = require('../package.json');
  return packageJson.version;
}

function downloadFile(url, dest) {
  return new Promise((resolve, reject) => {
    const follow = (url, redirects = 0) => {
      if (redirects > 5) {
        reject(new Error('Too many redirects'));
        return;
      }

      https.get(url, (res) => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          follow(res.headers.location, redirects + 1);
          return;
        }

        if (res.statusCode !== 200) {
          reject(new Error(`Download failed with status: ${res.statusCode}`));
          return;
        }

        const file = fs.createWriteStream(dest);
        res.pipe(file);
        file.on('finish', () => {
          file.close();
          resolve();
        });
        file.on('error', reject);
      }).on('error', reject);
    };

    follow(url);
  });
}

async function main() {
  const platformKey = getPlatformKey();
  const binaryName = getBinaryName(platformKey);
  const version = getPackageVersion();

  console.log(`Installing k8s-gpu-mcp-server v${version} for ${platformKey}...`);

  // Create bin directory
  const binDir = path.join(__dirname, '..', 'bin');
  if (!fs.existsSync(binDir)) {
    fs.mkdirSync(binDir, { recursive: true });
  }

  // Download URL
  const downloadUrl =
    `https://github.com/${REPO}/releases/download/v${version}/${binaryName}`;
  const destPath = path.join(
    binDir,
    BINARY_NAME + (process.platform === 'win32' ? '.exe' : '')
  );

  try {
    console.log(`Downloading from: ${downloadUrl}`);
    await downloadFile(downloadUrl, destPath);

    // Make executable on Unix
    if (process.platform !== 'win32') {
      fs.chmodSync(destPath, 0o755);
    }

    console.log(`Successfully installed to: ${destPath}`);
  } catch (error) {
    console.error(`Failed to download binary: ${error.message}`);
    console.error('');
    console.error('You may need to:');
    console.error('1. Check if the release exists on GitHub');
    console.error(
      '2. Build from source: https://github.com/ArangoGutierrez/k8s-gpu-mcp-server'
    );
    process.exit(1);
  }
}

main();

