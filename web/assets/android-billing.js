// Set-up android billing.
async function initAndroidBilling() {
  if (!("getDigitalGoodsService" in window)) {
    // Not in TWA / Play context — hide purchase UI
    document
      .querySelectorAll("[data-billing]")
      .forEach((el) => (el.hidden = true));
    return;
  }

  const service = await window.getDigitalGoodsService(
    "https://play.google.com/billing",
  );
  window.__billingService = service;

  // Fetch SKU details and update server-rendered price elements
  const skus = await service.getDetails(["premium_monthly", "premium_yearly"]);
  skus.forEach((sku) => {
    const el = document.querySelector(`[data-sku="${sku.itemId}"]`);
    if (el) el.textContent = sku.price.value + " " + sku.price.currency;
  });
}

// Triggered by an HTMX-rendered button click
async function androidPurchase(sku) {
  const service = window.__billingService;

  const paymentMethod = {
    supportedMethods: "https://play.google.com/billing",
    data: { sku },
  };

  const request = new PaymentRequest([paymentMethod], {
    total: { label: "Total", amount: { currency: "USD", value: "0" } },
  });

  try {
    const response = await request.show();
    const { purchaseToken } = response.details;

    // Send token to your Go server via HTMX-style fetch
    // The server returns an HTML fragment confirming success
    const res = await fetch("/billing/verify", {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: `sku=${sku}&token=${purchaseToken}`,
    });

    document.getElementById("billing-result").innerHTML = await res.text();
    await response.complete("success");
  } catch (err) {
    await response?.complete("fail");
  }
}
