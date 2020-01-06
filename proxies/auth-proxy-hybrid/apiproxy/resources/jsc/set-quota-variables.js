context.setVariable('quota.identifier', request.body.asJSON.identifier);
context.setVariable("quota.allow", request.body.asJSON.allow);
context.setVariable("quota.interval", request.body.asJSON.interval);
context.setVariable("quota.unit", request.body.asJSON.timeUnit);
context.setVariable("quota.weight", request.body.asJSON.weight);
