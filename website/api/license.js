// Licence keys.
//   GET  /api/license?session_id=…   the key for a checkout that has just been paid
//   POST /api/license {key}          a fresh key for a subscription that is still paid up;
//                                    panels ask for this once a day
import { keyForSubscription, json, problem, readKey, stripe, subscriptionActive } from "./_lib.js";

export async function GET(request) {
  const session = new URL(request.url).searchParams.get("session_id") ?? "";
  if (!/^cs_[A-Za-z0-9_]+$/.test(session)) return problem("That link is not complete.");
  try {
    const checkout = await stripe("GET", `/checkout/sessions/${session}?expand[]=subscription`);
    if (checkout.status !== "complete" || !checkout.subscription) return problem("That payment has not gone through yet.", 402);
    return json({ key: keyForSubscription(checkout.subscription), plan: checkout.subscription.metadata?.plan });
  } catch (error) {
    return problem(error.message, 502);
  }
}

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
    const subscription = await stripe("GET", `/subscriptions/${details.sub}`);
    if (!subscriptionActive(subscription)) return problem("This subscription has ended.", 402);
    return json({ key: keyForSubscription(subscription) });
  } catch (error) {
    return problem(error.message, 502);
  }
}
