// ensures that in the response, used <= allowed
// and exceeded is a count of the excess of used > allow
// assumes that allow is set arbitrarily high in the actual policy
var used = context.getVariable("ratelimit.DistributedQuota.used.count")
var allowed = context.getVariable("quota.allow")
if (used > allowed) {
    var exceeded = used - allowed
    context.setVariable("quota.used", allowed)
    context.setVariable("quota.exceeded", exceeded.toFixed(0))
} else {
    var exceeded = 0
    context.setVariable("quota.used", used)
    context.setVariable("quota.exceeded", exceeded.toFixed(0))
}
