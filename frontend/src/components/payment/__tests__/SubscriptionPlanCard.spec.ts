import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import { createPinia } from "pinia";
import { createI18n } from "vue-i18n";
import SubscriptionPlanCard from "../SubscriptionPlanCard.vue";

const i18n = createI18n({
  legacy: false,
  locale: "en",
  fallbackWarn: false,
  missingWarn: false,
  messages: {
    en: {
      payment: {
        days: "days",
        models: "Models",
        planCard: {
          quota: "Quota",
          rate: "Rate",
          unlimited: "Unlimited",
        },
        renewNow: "Renew",
        subscribeNow: "Subscribe now",
      },
    },
  },
});

const mountPlanCard = (groupPlatform: string) =>
  mount(SubscriptionPlanCard, {
    props: {
      plan: {
        id: 1,
        group_id: 10,
        group_platform: groupPlatform,
        name: "Pro",
        product_name: "GPT Pro Workbench",
        cover_image_url: "/assets/plans/pro.webp",
        detail_description: "Long product copy",
        price: 10,
        amount: 1000,
        features: ["Priority models"],
        rate_multiplier: 1,
        validity_days: 30,
        validity_unit: "day",
        supported_model_scopes: ["claude", "gemini_text", "gemini_image"],
        is_active: true,
      },
    },
    global: { plugins: [i18n, createPinia()] },
  });

describe("SubscriptionPlanCard", () => {
  it("does not show Antigravity model scopes for OpenAI plans", () => {
    const text = mountPlanCard("openai").text();

    expect(text).not.toContain("Claude");
    expect(text).not.toContain("Gemini");
    expect(text).not.toContain("Imagen");
  });

  it("shows model scopes for Antigravity plans", () => {
    const text = mountPlanCard("antigravity").text();

    expect(text).toContain("Claude");
    expect(text).toContain("Gemini");
    expect(text).toContain("Imagen");
  });

  it("renders product storefront fields and separates detail click from subscribe click", async () => {
    const wrapper = mountPlanCard("openai");

    expect(wrapper.text()).toContain("GPT Pro Workbench");
    expect(wrapper.find('[data-test="plan-cover-image"]').attributes("src")).toBe("/assets/plans/pro.webp");

    await wrapper.find('[data-test="plan-detail-trigger"]').trigger("click");
    expect(wrapper.emitted("details")?.[0]).toBeTruthy();
    expect(wrapper.emitted("select")).toBeUndefined();

    await wrapper.find('[data-test="plan-subscribe-button"]').trigger("click");
    expect(wrapper.emitted("select")?.[0]).toBeTruthy();
  });

  it("shows a platform-colored placeholder when the plan has no cover image", () => {
    const wrapper = mount(SubscriptionPlanCard, {
      props: {
        plan: {
          id: 2,
          group_id: 10,
          group_platform: "openai",
          name: "Basic",
          price: 5,
          features: [],
          rate_multiplier: 1,
          validity_days: 30,
          validity_unit: "day",
          for_sale: true,
          sort_order: 1,
          cover_image_url: "",
          detail_description: "",
          product_name: "",
        },
      },
      global: { plugins: [i18n, createPinia()] },
    });

    expect(wrapper.find('[data-test="plan-cover-placeholder"]').exists()).toBe(true);
    expect(wrapper.find('[data-test="plan-cover-image"]').exists()).toBe(false);
  });
});
