/**
 * Cloudflare Worker: Signed URL Reverse Proxy
 *
 * This worker acts as a reverse proxy that only accepts requests with valid signatures.
 *
 * URL Format: https://your-worker.workers.dev/?url=<encoded-target-url>&signature=<hmac-signature>&expires=<timestamp>
 *
 * Configuration:
 * Set these as environment variables in Cloudflare Workers dashboard:
 * - SIGNING_KEY: Your secret key for HMAC signing
 * - SIGNING_SALT: Additional salt for extra security (optional but recommended)
 */

// Configuration - Set these in Cloudflare Worker environment variables
const CONFIG = {
  // Maximum age of signed URLs in seconds (default: 1 hour)
  MAX_AGE: 3600,
  // Allowed domains (optional - remove or set to null to allow all)
  ALLOWED_DOMAINS: null, // e.g., ['example.com', 'cdn.example.com']
};

/**
 * Main request handler
 */
export default {
  async fetch(request, env) {
    try {
      // Only allow GET and HEAD requests
      if (!["GET", "HEAD"].includes(request.method)) {
        return new Response("Method not allowed", { status: 405 });
      }

      const url = new URL(request.url);

      // Extract query parameters
      const targetUrl = url.searchParams.get("url");
      const signature = url.searchParams.get("signature");
      const expires = url.searchParams.get("expires");

      // Validate required parameters
      if (!targetUrl || !signature || !expires) {
        console.error(
          JSON.stringify({
            message: "Missing required parameters",
            required: ["url", "signature", "expires"],
          }),
        );
        return new Response(
          JSON.stringify({
            error: "Missing required parameters",
            required: ["url", "signature", "expires"],
          }),
          {
            status: 400,
            headers: { "Content-Type": "application/json" },
          },
        );
      }

      // Check if signing key is configured
      if (!env.REVERSEPROXY_KEY) {
        console.error(
          JSON.stringify({
            message: "REVERSEPROXY_KEY not configured",
          }),
        );
        return new Response("Service configuration error", { status: 500 });
      }

      // Validate the signature
      const isValid = await validateSignature(
        targetUrl,
        signature,
        expires,
        env.REVERSEPROXY_KEY,
        env.REVERSEPROXY_SALT || "",
      );

      if (!isValid) {
        console.error(
          JSON.stringify({
            message: "Invalid or expired signature",
          }),
        );
        return new Response(
          JSON.stringify({ error: "Invalid or expired signature" }),
          {
            status: 403,
            headers: { "Content-Type": "application/json" },
          },
        );
      }

      // Check if URL has expired
      const expiresTimestamp = parseInt(expires, 10);
      const now = Math.floor(Date.now() / 1000);

      if (expiresTimestamp < now) {
        console.error(
          JSON.stringify({
            message: "URL has expired",
          }),
        );
        return new Response(JSON.stringify({ error: "URL has expired" }), {
          status: 403,
          headers: { "Content-Type": "application/json" },
        });
      }

      // Check if expires timestamp is too far in the future
      if (expiresTimestamp > now + CONFIG.MAX_AGE) {
        console.error(
          JSON.stringify({
            message: "Expiration time too far in future",
          }),
        );
        return new Response(
          JSON.stringify({ error: "Expiration time too far in future" }),
          {
            status: 403,
            headers: { "Content-Type": "application/json" },
          },
        );
      }

      // Validate target URL
      let targetUrlObj;
      try {
        targetUrlObj = new URL(targetUrl);
      } catch (e) {
        console.error(
          JSON.stringify({
            message: "Invalid target URL",
            url: targetUrl,
          }),
        );
        return new Response(JSON.stringify({ error: "Invalid target URL" }), {
          status: 400,
          headers: { "Content-Type": "application/json" },
        });
      }

      // Check allowed domains if configured
      if (CONFIG.ALLOWED_DOMAINS && CONFIG.ALLOWED_DOMAINS.length > 0) {
        const hostname = targetUrlObj.hostname;
        const isAllowed = CONFIG.ALLOWED_DOMAINS.some(
          (domain) => hostname === domain || hostname.endsWith("." + domain),
        );

        if (!isAllowed) {
          console.error(
            JSON.stringify({
              message: "Domain not allowed",
              domain: hostname,
            }),
          );
          return new Response(JSON.stringify({ error: "Domain not allowed" }), {
            status: 403,
            headers: { "Content-Type": "application/json" },
          });
        }
      }

      // Proxy the request
      console.log(
        JSON.stringify({
          message: "proxying request for: " + targetUrl,
          method: request.method,
          url: targetUrl,
        }),
      );

      const proxyResponse = await fetch(targetUrl, {
        method: request.method,
        headers: {
          "User-Agent": "Foragd (+https://foragd.app/policies/bot)",
          Accept: request.headers.get("Accept") || "*/*",
          "Accept-Encoding":
            request.headers.get("Accept-Encoding") || "gzip, deflate",
        },
        redirect: "follow",
      });

      // Create response with appropriate headers
      const responseHeaders = new Headers(proxyResponse.headers);

      // Add CORS headers if needed
      responseHeaders.set("Access-Control-Allow-Origin", "*");
      responseHeaders.set("X-Proxied-By", "Cloudflare-Worker");

      // Remove headers that shouldn't be proxied
      responseHeaders.delete("set-cookie");
      responseHeaders.delete("Set-Cookie");

      return new Response(proxyResponse.body, {
        status: proxyResponse.status,
        statusText: proxyResponse.statusText,
        headers: responseHeaders,
      });
    } catch (error) {
      console.error(
        JSON.stringify({
          message: "proxy error",
          error: error,
        }),
      );
      return new Response(JSON.stringify({ error: "Internal server error" }), {
        status: 500,
        headers: { "Content-Type": "application/json" },
      });
    }
  },
};

/**
 * Validate HMAC signature
 */
async function validateSignature(url, signature, expires, key, salt) {
  const message = `${url}|${expires}|${salt}`;
  const expectedSignature = await generateSignature(message, key);

  // Constant-time comparison to prevent timing attacks
  return constantTimeEqual(signature, expectedSignature);
}

/**
 * Generate HMAC-SHA256 signature
 */
async function generateSignature(message, key) {
  const encoder = new TextEncoder();
  const keyData = encoder.encode(key);
  const messageData = encoder.encode(message);

  const cryptoKey = await crypto.subtle.importKey(
    "raw",
    keyData,
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"],
  );

  const signature = await crypto.subtle.sign("HMAC", cryptoKey, messageData);

  // Convert to hex string
  return Array.from(new Uint8Array(signature))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

/**
 * Constant-time string comparison to prevent timing attacks
 */
function constantTimeEqual(a, b) {
  if (a.length !== b.length) {
    return false;
  }

  let result = 0;
  for (let i = 0; i < a.length; i++) {
    result |= a.charCodeAt(i) ^ b.charCodeAt(i);
  }

  return result === 0;
}
