package repository

const usageLogChargedCostExpr = "CASE WHEN COALESCE(billed_cost, 0) > 0 THEN billed_cost ELSE actual_cost + COALESCE(billing_surcharge_cost, 0) END"
const usageLogChargedCostExprUL = "CASE WHEN COALESCE(ul.billed_cost, 0) > 0 THEN ul.billed_cost ELSE ul.actual_cost + COALESCE(ul.billing_surcharge_cost, 0) END"
const usageLogChargedCostExprU = "CASE WHEN COALESCE(u.billed_cost, 0) > 0 THEN u.billed_cost ELSE u.actual_cost + COALESCE(u.billing_surcharge_cost, 0) END"
