/**
 * Cloudflare Worker: Signed URL Browser Renderer
 *
 * This worker acts as a signed-URL-gated browser renderer using Cloudflare's
 * Browser Rendering API to return the full rendered HTML of a target URL.
 *
 * URL Format: https://your-worker.workers.dev/?url=<encoded-target-url>&signature=<hmac-signature>&expires=<timestamp>
 *
 * Configuration (Wrangler / Dashboard environment variables):
 * - REVERSEPROXY_KEY:            Secret key for HMAC signing
 * - REVERSEPROXY_SALT:           Optional salt for extra security
 * - ACCESS_TOKEN_CLIENT_ID:      Cloudflare Zero Trust service token client ID
 * - ACCESS_TOKEN_CLIENT_SECRET:  Cloudflare Zero Trust service token client secret
 *
 * wrangler.toml requirements:
 * [browser]
 * binding = "BROWSER"
 */

import puppeteer from "@cloudflare/puppeteer";

// Configuration
const CONFIG = {
  // Maximum age of signed URLs in seconds (default: 1 hour)
  MAX_AGE: 3600,
  // Allowed domains (null = allow all)
  ALLOWED_DOMAINS: null, // e.g. ['example.com', 'cdn.example.com']
  // Puppeteer navigation timeout in milliseconds
  NAVIGATION_TIMEOUT: 30_000,
  // How long to wait for network to be idle after navigation (ms)
  NETWORK_IDLE_TIMEOUT: 2_000,
};

export default {
  async fetch(request, env) {
    try {
      // Only allow GET and HEAD requests
      if (!["GET", "HEAD"].includes(request.method)) {
        return new Response("Method not allowed", { status: 405 });
      }

      const version =
        (typeof process !== "undefined" && process.env?.VERSION) ?? "dev";
      const url = new URL(request.url);

      // ── Extract query parameters ──────────────────────────────────────────
      const targetUrl = url.searchParams.get("url");
      const signature = url.searchParams.get("signature");
      const expires = url.searchParams.get("expires");

      if (!targetUrl || !signature || !expires) {
        return jsonError(400, "Missing required parameters", {
          required: ["url", "signature", "expires"],
        });
      }

      // ── Verify environment config ─────────────────────────────────────────
      if (!env.ACCESS_TOKEN_CLIENT_ID || !env.ACCESS_TOKEN_CLIENT_SECRET) {
        console.error(
          JSON.stringify({ message: "Access token env vars not configured" }),
        );
        return new Response("Service configuration error", { status: 500 });
      }

      if (!env.REVERSEPROXY_KEY) {
        console.error(
          JSON.stringify({ message: "REVERSEPROXY_KEY not configured" }),
        );
        return new Response("Service configuration error", { status: 500 });
      }

      if (!env.BROWSER) {
        console.error(
          JSON.stringify({ message: "BROWSER binding not configured" }),
        );
        return new Response(
          "Service configuration error — BROWSER binding missing",
          { status: 500 },
        );
      }

      // ── Validate signature ────────────────────────────────────────────────
      const isValid = await validateSignature(
        targetUrl,
        signature,
        expires,
        env.REVERSEPROXY_KEY,
        env.REVERSEPROXY_SALT || "",
      );

      if (!isValid) {
        console.error(
          JSON.stringify({ message: "Invalid or expired signature" }),
        );
        return jsonError(403, "Invalid or expired signature");
      }

      // ── Check expiry window ───────────────────────────────────────────────
      const expiresTimestamp = parseInt(expires, 10);
      const now = Math.floor(Date.now() / 1000);

      if (expiresTimestamp < now) {
        return jsonError(403, "URL has expired");
      }

      if (expiresTimestamp > now + CONFIG.MAX_AGE) {
        return jsonError(403, "Expiration time too far in future");
      }

      // ── Validate target URL ───────────────────────────────────────────────
      let targetUrlObj;
      try {
        targetUrlObj = new URL(targetUrl);
      } catch {
        return jsonError(400, "Invalid target URL");
      }

      // Must be http(s)
      if (!["http:", "https:"].includes(targetUrlObj.protocol)) {
        return jsonError(400, "Target URL must use http or https");
      }

      // ── Domain allowlist ──────────────────────────────────────────────────
      if (CONFIG.ALLOWED_DOMAINS?.length) {
        const hostname = targetUrlObj.hostname;
        const isAllowed = CONFIG.ALLOWED_DOMAINS.some(
          (d) => hostname === d || hostname.endsWith("." + d),
        );
        if (!isAllowed) {
          return jsonError(403, "Domain not allowed");
        }
      }

      // ── Browser rendering ─────────────────────────────────────────────────
      console.log(JSON.stringify({ message: "Rendering URL", url: targetUrl }));

      if (request.method === "HEAD") {
        // Skip expensive rendering for HEAD requests
        return new Response(null, {
          status: 200,
          headers: {
            "Content-Type": "text/html; charset=utf-8",
            "X-Rendered-By": "Cloudflare-Browser-Rendering",
          },
        });
      }

      const html = await renderWithBrowser(env.BROWSER, targetUrl, {
        clientId: env.ACCESS_TOKEN_CLIENT_ID,
        clientSecret: env.ACCESS_TOKEN_CLIENT_SECRET,
        version,
      });

      return new Response(html, {
        status: 200,
        headers: {
          "Content-Type": "text/html; charset=utf-8",
          "Access-Control-Allow-Origin": "*",
          "X-Rendered-By": "Cloudflare-Browser-Rendering",
          "X-Source-URL": targetUrl,
          "Cache-Control": "no-store",
        },
      });
    } catch (error) {
      console.error(
        JSON.stringify({ message: "Worker error", error: String(error) }),
      );
      return jsonError(500, "Internal server error");
    }
  },
};

// ─────────────────────────────────────────────────────────────────────────────
// Browser rendering
// ─────────────────────────────────────────────────────────────────────────────

/**
 * Launch a Puppeteer browser via the Browser Rendering binding, navigate to
 * the target URL (injecting Zero Trust service-token headers via a request
 * interception), wait for the network to settle, then return the full outer HTML.
 */
async function renderWithBrowser(
  browserBinding,
  targetUrl,
  { clientId, clientSecret, version },
) {
  let browser;

  try {
    browser = await puppeteer.launch(browserBinding);
    const page = await browser.newPage();

    // ── Inject Zero Trust service-token headers via CDP ───────────────────
    const cdpSession = await page.createCDPSession();
    await cdpSession.send("Network.enable");
    await cdpSession.send("Network.setExtraHTTPHeaders", {
      headers: {
        "CF-Access-Client-Id": clientId,
        "CF-Access-Client-Secret": clientSecret,
        "User-Agent": `Foragd/${version} (+https://foragd.app/policies/bot)`,
      },
    });

    // ── Navigate ──────────────────────────────────────────────────────────
    await page.goto(targetUrl, {
      waitUntil: "networkidle2",
      timeout: CONFIG.NAVIGATION_TIMEOUT,
    });

    // ── Extract full rendered HTML ────────────────────────────────────────
    const html = await page.content();

    return html;
  } finally {
    if (browser) {
      await browser.close();
    }
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Signature helpers (unchanged from original)
// ─────────────────────────────────────────────────────────────────────────────

async function validateSignature(url, signature, expires, key, salt) {
  const message = `${url}|${expires}|${salt}`;
  const expectedSignature = await generateSignature(message, key);
  return constantTimeEqual(signature, expectedSignature);
}

async function generateSignature(message, key) {
  const encoder = new TextEncoder();
  const cryptoKey = await crypto.subtle.importKey(
    "raw",
    encoder.encode(key),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"],
  );
  const sig = await crypto.subtle.sign(
    "HMAC",
    cryptoKey,
    encoder.encode(message),
  );
  return Array.from(new Uint8Array(sig))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

function constantTimeEqual(a, b) {
  if (a.length !== b.length) return false;
  let result = 0;
  for (let i = 0; i < a.length; i++) {
    result |= a.charCodeAt(i) ^ b.charCodeAt(i);
  }
  return result === 0;
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

function jsonError(status, message, extra = {}) {
  return new Response(JSON.stringify({ error: message, ...extra }), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}
