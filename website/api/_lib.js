// Shared by the payment functions. Files starting with "_" are not served as functions.
//
// Two settings are needed in Vercel (Project, Settings, Environment Variables):
//   STRIPE_SECRET_KEY     the secret key from the Stripe dashboard (sk_live_… or sk_test_…)
//   LICENSE_PRIVATE_KEY   the private key that signs licence keys, as PEM text
// Stripe itself is the only record of who has paid: nothing else is stored.
import { createPrivateKey, sign } from "node:crypto";

export const siteUrl = "https://www.consolry.com";

/** The paid plans and how many nodes each covers. Host covers 5, and more can be bought. */
export const paidPlans = { plus: { nodes: Infinity }, pro: { nodes: Infinity }, host: { nodes: 5 } };

export function json(body, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json", "Cache-Control": "no-store" } });
}

export function problem(message, status = 400) {
  return json({ error: message }, status);
}

/** Calls Stripe's API. Body values are sent the way Stripe expects, as a form. */
export async function stripe(method, path, values) {
  const key = process.env.STRIPE_SECRET_KEY;
  if (!key) throw new Error("Payments are not set up yet.");
  const body = values ? new URLSearchParams(flatten(values)) : undefined;
  const response = await fetch(`https://api.stripe.com/v1${path}`, {
    method,
    headers: { Authorization: `Bearer ${key}`, "Content-Type": "application/x-www-form-urlencoded" },
    body,
  });
  const data = await response.json();
  if (!response.ok) throw new Error(data.error?.message ?? `Stripe answered ${response.status}`);
  return data;
}

// Stripe wants nested values as line_items[0][price]=… pairs.
function flatten(value, prefix = "", out = {}) {
  for (const [key, item] of Object.entries(value)) {
    const name = prefix ? `${prefix}[${key}]` : key;
    if (item !== null && typeof item === "object") flatten(item, name, out);
    else if (item !== undefined) out[name] = String(item);
  }
  return out;
}

/** Finds the Stripe price made by scripts/stripe-setup.mjs, such as "plus_month". */
export async function price(lookupKey) {
  const found = await stripe("GET", `/prices?active=true&lookup_keys[]=${encodeURIComponent(lookupKey)}`);
  if (!found.data?.length) throw new Error(`There is no Stripe price called ${lookupKey}. Run the setup script first.`);
  return found.data[0].id;
}

const base64url = (data) => Buffer.from(data).toString("base64url");

/**
 * Makes a licence key: the details, then a signature only this site can make.
 * The panel checks the signature with the public half of the key, which is built into it.
 */
export function licenceKey(details) {
  const pem = process.env.LICENSE_PRIVATE_KEY;
  if (!pem) throw new Error("Licence keys are not set up yet.");
  const payload = Buffer.from(JSON.stringify(details));
  const signature = sign(null, payload, createPrivateKey(pem.replace(/\\n/g, "\n")));
  return `CONSOLRY-${base64url(payload)}.${base64url(signature)}`;
}

/** Reads a licence key's details. The signature is checked by the panel, not here. */
export function readKey(key) {
  const match = /^CONSOLRY-([A-Za-z0-9_-]+)\.([A-Za-z0-9_-]+)$/.exec(String(key ?? "").trim());
  if (!match) return null;
  try {
    return JSON.parse(Buffer.from(match[1], "base64url").toString());
  } catch {
    return null;
  }
}

/** A subscription that is paid up, or within Stripe's retry period for a failed payment. */
export const subscriptionActive = (subscription) => ["active", "trialing", "past_due"].includes(subscription.status);

/** The licence key for a subscription, valid until a week after the paid period ends. */
export function keyForSubscription(subscription) {
  const plan = subscription.metadata?.plan;
  if (!paidPlans[plan]) throw new Error("This subscription is not for a Consolry plan.");
  const nodes = Number(subscription.metadata?.nodes) || paidPlans[plan].nodes;
  const end = subscription.items?.data?.[0]?.current_period_end ?? subscription.current_period_end;
  return licenceKey({
    sub: subscription.id,
    cus: typeof subscription.customer === "string" ? subscription.customer : subscription.customer?.id,
    plan,
    nodes: Number.isFinite(nodes) ? nodes : 0,
    iat: Math.floor(Date.now() / 1000),
    exp: end + 7 * 24 * 3600,
  });
}
