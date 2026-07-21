import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import { createPinia } from "pinia";
import { createI18n } from "vue-i18n";
import type { SubscriptionPlan } from "@/types/payment";
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
          featured: "Recommended",
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

const mountPlanCard = (groupPlatform: string, overrides: Partial<SubscriptionPlan> = {}) =>
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
        ...overrides,
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

  it("shows storefront badges and uses the display platform when configured", () => {
    const wrapper = mount(SubscriptionPlanCard, {
      props: {
        plan: {
          id: 3,
          group_id: 10,
          group_platform: "openai",
          storefront_platform: "image",
          storefront_featured: true,
          storefront_badge: "Hot",
          name: "Image Pack",
          price: 5,
          features: [],
          rate_multiplier: 1,
          validity_days: 1,
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

    const badges = wrapper.findAll('[data-test="plan-storefront-badge"]').map(node => node.text());
    expect(badges).toContain("payment.planCard.featured");
    expect(badges).toContain("Hot");
    expect(wrapper.text()).toContain("图片");
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

  it("uses the configured currency symbol while preserving USD for legacy plans", () => {
    const cnyPlan = mountPlanCard("openai", { currency: "CNY", original_price: 20 }).text();

    expect(cnyPlan).toContain("¥10CNY");
    expect(cnyPlan).toContain("¥20CNY");
    expect(mountPlanCard("openai", { currency: "USD" }).text()).toContain("$10USD");
    expect(mountPlanCard("openai", { currency: "" }).text()).toContain("$10");
  });
});
