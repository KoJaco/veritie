// app/api/schma/token/route.ts
// Next.js App Router (Node runtime). If you want Edge, swap fetch() for undici and avoid Node-only APIs.

export const runtime = "nodejs"; // or "edge" if your upstream supports it

export async function POST(req: Request) {
    // Option A: zero-body mint (key is app-scoped)
    const upstream = await fetch("https://api.schma.com/v1/tokens/ws", {
        method: "POST",
        headers: {
            "content-type": "application/json",
            "x-api-key": process.env.SCHMA_SECRET_KEY!, // <-- server-only
        },
        // no body required; key implies app_id
    });

    if (!upstream.ok) {
        const text = await upstream.text();
        return new Response(text, {
            status: upstream.status,
            headers: { "content-type": "application/json" },
        });
    }

    // { token: string, expires_in: number, ws_session_id?: string }
    const data = await upstream.json();
    return new Response(JSON.stringify(data), {
        status: 200,
        headers: { "content-type": "application/json" },
    });
}
