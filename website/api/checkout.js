// Starts a Stripe checkout for a paid plan and sends the buyer there.
import { paidPlans, price, problem, json, siteUrl, stripe } from "./_lib.js";

export async function POST(request) {
  let input;
  try {
    input = await request.json();
  } catch {
    return problem("Send the plan as JSON.");
  }
  const plan = String(input.plan ?? "");
  const period = input.period === "year" ? "year" : "month";
  if (!paidPlans[plan]) return problem("Choose Plus, Pro or Host.");
  const nodes = plan === "host" ? Math.max(5, Math.min(1000, Math.floor(Number(input.nodes) || 5))) : 0;

  try {
    const items = { 0: { price: await price(`${plan}_${period}`), quantity: 1 } };
    if (plan === "host" && nodes > 5) items[1] = { price: await price(`node_${period}`), quantity: nodes - 5 };
    const session = await stripe("POST", "/checkout/sessions", {
      mode: "subscription",
      line_items: items,
      success_url: `${siteUrl}/license?session_id={CHECKOUT_SESSION_ID}`,
      cancel_url: `${siteUrl}/pricing`,
      allow_promotion_codes: true,
      billing_address_collection: "auto",
      // Kept on the subscription, so the licence key can be made again from it each month.
      subscription_data: { metadata: { plan, nodes: plan === "host" ? nodes : "" } },
    });
    return json({ url: session.url });
  } catch (error) {
    return problem(error.message, 502);
  }
}
