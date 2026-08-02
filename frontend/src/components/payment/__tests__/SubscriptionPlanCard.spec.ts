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
        weeks: "weeks",
        months: "months",
        years: "years",
        perMonth: "month",
        perYear: "year",
        models: "Models",
        planCard: {
          dailyLimit: "Daily limit",
          featured: "Recommended",
          monthlyLimit: "Monthly limit",
          quota: "Quota",
          rate: "Rate",
          unlimited: "Unlimited",
          weeklyLimit: "Weekly limit",
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

  // Issue 4607：管理端保存的单位是复数（months/weeks），此前用户侧只匹配单数
  // 'month'，「1 个月」的套餐卡片被显示成「1天」。测试环境的 vue-i18n 为
  // runtime-only 构建，t() 原样返回 key，故按 key 断言单位分支。
  it("renders plural admin-form validity units instead of mislabeled days (issue 4607)", () => {
    expect(mountPlanCard("openai", { validity_days: 1, validity_unit: "months" }).text()).toContain("/ payment.perMonth");
    expect(mountPlanCard("openai", { validity_days: 3, validity_unit: "months" }).text()).toContain("/ 3payment.months");
    expect(mountPlanCard("openai", { validity_days: 2, validity_unit: "weeks" }).text()).toContain("/ 2payment.weeks");
    expect(mountPlanCard("openai", { validity_days: 30, validity_unit: "day" }).text()).toContain("/ 30payment.days");
  });

  it("preserves legacy yearly validity labels", () => {
    expect(mountPlanCard("openai", { validity_days: 1, validity_unit: "year" }).text()).toContain("/ payment.perYear");
    expect(mountPlanCard("openai", { validity_days: 2, validity_unit: "years" }).text()).toContain("/ 2payment.years");
  });

  it("uses the configured currency symbol while preserving USD for legacy plans", () => {
    const cnyPlan = mountPlanCard("openai", { currency: "CNY", original_price: 20 }).text();

    expect(cnyPlan).toContain("¥10CNY");
    expect(cnyPlan).toContain("¥20CNY");
    expect(mountPlanCard("openai", { currency: "USD" }).text()).toContain("$10USD");
    expect(mountPlanCard("openai", { currency: "" }).text()).toContain("$10");
  });

  it("does not render zero quota limits as purchasable limits", () => {
    const text = mountPlanCard("openai", {
      daily_limit_usd: 0,
      weekly_limit_usd: 0,
      monthly_limit_usd: 249,
    }).text();

    expect(text).toContain("payment.planCard.monthlyLimit");
    expect(text).toContain("$249");
    expect(text).not.toContain("Daily limit$0");
    expect(text).not.toContain("Weekly limit$0");
    expect(text).not.toContain("$0");
  });

  it.each([
    ["long Chinese", "企业全球加速专业订阅套餐（含高级模型与优先支持）"],
    ["long English", "Enterprise Global Acceleration Subscription with Priority Support"],
    ["unbroken token", "EnterpriseGlobalAccelerationSubscriptionWithPrioritySupport1234567890"],
  ])("keeps the full %s plan title accessible in a bounded two-line area", (_label, name) => {
    const wrapper = mountPlanCard("openai", { name, product_name: "" });
    const title = wrapper.get("h3");

    expect(title.text()).toBe(name);
    expect(title.attributes("title")).toBe(name);
    expect(title.classes()).toEqual(expect.arrayContaining([
      "min-w-0",
      "h-12",
      "break-words",
      "line-clamp-2",
      "[overflow-wrap:anywhere]",
    ]));
    expect(title.classes()).not.toContain("truncate");
  });

  it("keeps title, badge, price, description, and purchase action in separate bounded regions", () => {
    const wrapper = mountPlanCard("openai", {
      name: "Enterprise Global Acceleration Subscription with Priority Support",
      product_name: "",
      price: 123.45,
      currency: "USD",
      description: "Includes advanced models and priority support.",
    });
    const title = wrapper.get("h3");
    const badge = wrapper.findAll("span").find((node) => node.text() === "OpenAI");
    const price = wrapper.findAll("span").find((node) => node.text() === "123.45");

    expect(title.element.parentElement?.classList).toContain("min-w-0");
    expect(title.element.parentElement?.classList).toContain("flex-1");
    expect(badge?.classes()).toContain("shrink-0");
    expect([...(badge?.element.parentElement?.classList ?? [])]).toEqual(expect.arrayContaining([
      "flex",
      "items-center",
      "justify-end",
    ]));
    expect(badge?.element.parentElement?.textContent).toContain("/ 30payment.days");
    expect(badge?.element.parentElement?.parentElement?.classList).toContain("shrink-0");
    expect(price?.element.parentElement?.parentElement?.classList).toContain("shrink-0");
    expect(wrapper.get("p").text()).toBe("Includes advanced models and priority support.");
    expect(wrapper.get("button").text()).toBe("payment.subscribeNow");
  });

  it("keeps short plan titles compact and aligned", () => {
    const wrapper = mountPlanCard("openai", { name: "Pro", product_name: "", description: "" });
    const title = wrapper.get("h3");
    const badge = wrapper.findAll("span").find((node) => node.text() === "OpenAI");

    expect(title.text()).toBe("Pro");
    expect(title.attributes("title")).toBe("Pro");
    expect(title.classes()).toEqual(expect.arrayContaining(["text-base", "font-bold", "h-12"]));
    expect([...(badge?.element.parentElement?.classList ?? [])]).toEqual(expect.arrayContaining([
      "flex",
      "items-center",
      "justify-end",
    ]));
    expect(badge?.element.parentElement?.textContent).toContain("/ 30payment.days");
  });
});
