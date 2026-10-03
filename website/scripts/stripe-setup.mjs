// Creates Consolry's products and prices in Stripe, and the page customers use to manage
// their subscription. Safe to run again: anything that already exists is left alone.
//
// In PowerShell:
//   $env:STRIPE_SECRET_KEY = "sk_test_…"; node scripts/stripe-setup.mjs
// Run it once with a test key, try a purchase, then once more with the live key.
import { stripe } from "../api/_lib.js";

const products = [
  { plan: "plus", name: "Consolry Plus", month: 600 },
  { plan: "pro", name: "Consolry Pro", month: 1900 },
  { plan: "host", name: "Consolry Host", month: 4900, about: "Covers 5 nodes." },
  { plan: "node", name: "Consolry Host extra node", month: 800, about: "Each node beyond the first 5." },
];

for (const product of products) {
  const keys = [`${product.plan}_month`, `${product.plan}_year`];
  const existing = await stripe("GET", `/prices?active=true&${keys.map((key) => `lookup_keys[]=${key}`).join("&")}`);
  if (existing.data.length === keys.length) {
    console.log(`${product.name}: already set up`);
    continue;
  }
  const made = await stripe("POST", "/products", { name: product.name, description: product.about, metadata: { plan: product.plan } });
  // A year costs ten months.
  for (const [interval, amount] of [
    ["month", product.month],
    ["year", product.month * 10],
  ]) {
    await stripe("POST", "/prices", {
      product: made.id,
      currency: "usd",
      unit_amount: amount,
      recurring: { interval },
      lookup_key: `${product.plan}_${interval}`,
      transfer_lookup_key: true,
    });
  }
  console.log(`${product.name}: created`);
}

const portals = await stripe("GET", "/billing_portal/configurations?is_default=true&active=true");
if (portals.data.length === 0) {
  await stripe("POST", "/billing_portal/configurations", {
    business_profile: { headline: "Manage your Consolry subscription" },
    features: {
      invoice_history: { enabled: true },
      payment_method_update: { enabled: true },
      customer_update: { enabled: true, allowed_updates: { 0: "email", 1: "address" } },
      subscription_cancel: { enabled: true, mode: "at_period_end" },
    },
  });
  console.log("Customer billing page: created");
} else {
  console.log("Customer billing page: already set up");
}
console.log("Done.");
