// Opens Stripe's own page for changing a card, downloading invoices or cancelling.
// The licence key identifies the subscription, so no account on this site is needed.
import { json, problem, readKey, siteUrl, stripe } from "./_lib.js";

export async function POST(request) {
  let input;
  try {
    input = await request.json();
  } catch {
    return problem("Send the key as JSON.");
  }
  const details = readKey(input.key);
  if (!details?.sub || !/^sub_[A-Za-z0-9]+$/.test(details.sub)) return problem("That is not a Consolry licence key.");
  try {
    // The customer is looked up from the subscription rather than trusted from the key.
    const subscription = await stripe("GET", `/subscriptions/${details.sub}`);
    const portal = await stripe("POST", "/billing_portal/sessions", { customer: subscription.customer, return_url: `${siteUrl}/license` });
    return json({ url: portal.url });
  } catch (error) {
    return problem(error.message, 502);
  }
}
