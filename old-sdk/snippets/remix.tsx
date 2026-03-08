// app/routes/schma.token.ts
// Route module that exposes a POST action at /schma/token

import type { ActionFunctionArgs } from "react-router"; // Remix v2 uses react-router types

export async function action({ request }: ActionFunctionArgs) {
    if (request.method !== "POST") {
        return { error: "Method Not Allowed" } as const; // 200 by default; set Response if you want 405
    }

    // Option A: zero-body mint (key is app-scoped)
    const upstream = await fetch("https://api.schma.com/v1/tokens/ws", {
        method: "POST",
        headers: {
            "content-type": "application/json",
            "x-api-key": process.env.SCHMA_SECRET_KEY!, // <-- server-only
        },
        // no body
    });

    if (!upstream.ok) {
        const text = await upstream.text();
        // To set a non-200 status with RR data APIs, wrap in Response:
        return new Response(text, {
            status: upstream.status,
            headers: { "content-type": "application/json" },
        });
    }

    const data = await upstream.json(); // { token, expires_in, ws_session_id? }
    // Plain object is fine in Remix v2
    return data;
}
