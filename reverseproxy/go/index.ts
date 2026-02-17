// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

import { Container, getContainer, getRandom } from "@cloudflare/containers";
import { Env, Hono } from "hono";
import { env } from "cloudflare:workers";

export class ReverseProxy extends Container<Env> {
	// Port the container listens on (default: 8080)
    envVars = {
      FORAGD_REVERSEPROXY_PORT: env.FORAGD_REVERSEPROXY_PORT,
      FORAGD_REVERSEPROXY_HOST: env.FORAGD_REVERSEPROXY_HOST,
      FORAGD_REVERSEPROXY_READTIMEOUT: env.FORAGD_REVERSEPROXY_READTIMEOUT,
      FORAGD_REVERSEPROXY_WRITETIMEOUT: env.FORAGD_REVERSEPROXY_WRITETIMEOUT,
      FORAGD_REVERSEPROXY_IDLETIMEOUT: env.FORAGD_REVERSEPROXY_IDLETIMEOUT,
  };
	// Time before container sleeps due to inactivity (default: 30s)
	sleepAfter = "2m";

	// Optional lifecycle hooks
	override onStart() {
		console.log("Container successfully started");
	}

	override onStop() {
		console.log("Container successfully shut down");
	}

	override onError(error: unknown) {
		console.log("Container error:", error);
	}
}

// Create Hono app with proper typing for Cloudflare Workers
const app = new Hono<{
	Bindings: Env;
}>();


export default {
  async fetch(request: { url: string | URL; }, env: { MY_CONTAINER: any; }) {
    const pathname = new URL(request.url).pathname;

    // // If you want to route requests to a specific container,
    // // pass a unique container identifier to .get()

    // if (pathname.startsWith('/specific/')) {
    //   // In this case, each unique pathname will spawn a new container
    //   const container = env.MY_CONTAINER.getByName(pathname);
    //   return await container.fetch(request);
    // }

    // Note: this is a temporary method until built-in autoscaling and load balancing are added.
    // If you want to route to one of many containers (in this case 5), use the getRandom helper.
    // This load balances incoming requests across these container instances.
    let container = await getRandom(env.MY_CONTAINER, 5);
    return await container.fetch(request);
  },
};

// // Home route with available endpoints
// app.get("/", (c) => {
// 	return await container.fetch(c.req.raw);
// 	return c.text(
// 		"Available endpoints:\n" +
// 			"GET /container/<ID> - Start a container for each ID with a 2m timeout\n" +
// 			"GET /lb - Load balance requests over multiple containers\n" +
// 			"GET /error - Start a container that errors (demonstrates error handling)\n" +
// 			"GET /singleton - Get a single specific container instance",
// 	);
// });

// // Route requests to a specific container using the container ID
// app.get("/container/:id", async (c) => {
// 	const id = c.req.param("id");
// 	const containerId = c.env.MY_CONTAINER.idFromName(`/container/${id}`);
// 	const container = c.env.MY_CONTAINER.get(containerId);
// 	return await container.fetch(c.req.raw);
// });

// // Demonstrate error handling - this route forces a panic in the container
// app.get("/error", async (c) => {
// 	const container = getContainer(c.env.MY_CONTAINER, "error-test");
// 	return await container.fetch(c.req.raw);
// });

// // Load balance requests across multiple containers
// app.get("/lb", async (c) => {
// 	const container = await getRandom(c.env.MY_CONTAINER, 3);
// 	return await container.fetch(c.req.raw);
// });

// // Get a single container instance (singleton pattern)
// app.get("/singleton", async (c) => {
// 	const container = getContainer(c.env.MY_CONTAINER);
// 	return await container.fetch(c.req.raw);
// });

// export default app;
