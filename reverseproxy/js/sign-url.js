/**
 * URL Signing Utility
 *
 * Use this script to generate signed URLs for your Cloudflare Worker proxy.
 *
 * Usage:
 *   node sign-url.js <target-url> [ttl-seconds]
 *
 * Example:
 *   node sign-url.js "https://example.com/image.jpg" 3600
 */

const crypto = require('crypto')

// Configuration - should match your Cloudflare Worker settings
const FORAGD_REVERSEPROXY_KEY =
  process.env.FORAGD_REVERSEPROXY_KEY || 'your-secret-key-here'
const FORAGD_REVERSEPROXY_SALT =
  process.env.FORAGD_REVERSEPROXY_SALT || 'your-salt-here'
const FORAGD_REVERSEPROXY_WORKER_URL =
  process.env.FORAGD_REVERSEPROXY_WORKER_URL ||
  'https://your-worker.workers.dev'

/**
 * Generate HMAC-SHA256 signature
 */
function generateSignature(message, key) {
  return crypto.createHmac('sha256', key).update(message).digest('hex')
}

/**
 * Generate a signed URL
 */
function signUrl(targetUrl, ttlSeconds = 3600) {
  // Calculate expiration timestamp
  const expires = Math.floor(Date.now() / 1000) + ttlSeconds

  // Create signature message
  const message = `${targetUrl}|${expires}|${FORAGD_REVERSEPROXY_SALT}`

  // Generate signature
  const signature = generateSignature(message, FORAGD_REVERSEPROXY_KEY)

  // Build signed URL
  const workerUrl = new URL(FORAGD_REVERSEPROXY_WORKER_URL)
  workerUrl.searchParams.set('url', targetUrl)
  workerUrl.searchParams.set('expires', expires.toString())
  workerUrl.searchParams.set('signature', signature)

  return {
    signedUrl: workerUrl.toString(),
    targetUrl: targetUrl,
    expiresAt: new Date(expires * 1000).toISOString(),
    expiresIn: ttlSeconds,
    signature: signature,
  }
}

/**
 * Verify a signed URL (for testing)
 */
function verifySignature(targetUrl, signature, expires) {
  const message = `${targetUrl}|${expires}|${FORAGD_REVERSEPROXY_SALT}`
  const expectedSignature = generateSignature(message, FORAGD_REVERSEPROXY_KEY)

  return signature === expectedSignature
}

// CLI interface
if (require.main === module) {
  const args = process.argv.slice(2)

  if (args.length === 0 || args[0] === '--help' || args[0] === '-h') {
    console.log('URL Signing Utility for Cloudflare Worker Proxy')
    console.log('')
    console.log('Usage:')
    console.log('  node sign-url.js <target-url> [ttl-seconds]')
    console.log('')
    console.log('Arguments:')
    console.log('  target-url    The URL to proxy (required)')
    console.log('  ttl-seconds   Time-to-live in seconds (default: 3600)')
    console.log('')
    console.log('Environment Variables:')
    console.log('  FORAGD_REVERSEPROXY_KEY   Secret key for signing (required)')
    console.log(
      '  FORAGD_REVERSEPROXY_SALT  Additional salt for security (optional)'
    )
    console.log(
      '  FORAGD_REVERSEPROXY_WORKER_URL    Your Cloudflare Worker URL (required)'
    )
    console.log('')
    console.log('Example:')
    console.log(
      '  FORAGD_REVERSEPROXY_KEY=mysecret FORAGD_REVERSEPROXY_WORKER_URL=https://proxy.workers.dev \\'
    )
    console.log('    node sign-url.js "https://example.com/image.jpg" 7200')
    process.exit(0)
  }

  const targetUrl = args[0]
  const ttlSeconds = args[1] ? parseInt(args[1], 10) : 3600

  if (
    !FORAGD_REVERSEPROXY_KEY ||
    FORAGD_REVERSEPROXY_KEY === 'your-secret-key-here'
  ) {
    console.error('Error: FORAGD_REVERSEPROXY_KEY environment variable not set')
    console.error('Set it with: export FORAGD_REVERSEPROXY_KEY=your-secret-key')
    process.exit(1)
  }

  if (
    !FORAGD_REVERSEPROXY_WORKER_URL ||
    FORAGD_REVERSEPROXY_WORKER_URL === 'https://your-worker.workers.dev'
  ) {
    console.error(
      'Error: FORAGD_REVERSEPROXY_WORKER_URL environment variable not set'
    )
    console.error(
      'Set it with: export FORAGD_REVERSEPROXY_WORKER_URL=https://your-worker.workers.dev'
    )
    process.exit(1)
  }

  try {
    new URL(targetUrl)
  } catch (e) {
    console.error('Error: Invalid target URL')
    process.exit(1)
  }

  const result = signUrl(targetUrl, ttlSeconds)

  console.log('Signed URL generated successfully!')
  console.log('')
  console.log('Target URL:', result.targetUrl)
  console.log('Expires at:', result.expiresAt)
  console.log('Expires in:', result.expiresIn, 'seconds')
  console.log('')
  console.log('Signed URL:')
  console.log(result.signedUrl)
  console.log('')
  console.log('Signature:', result.signature)
}

// Export for use as a module
module.exports = {
  signUrl,
  verifySignature,
  generateSignature,
}
