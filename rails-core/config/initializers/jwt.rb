JWT_SECRET = ENV.fetch("JWT_SECRET", "development-secret")
JWT_ISSUER = ENV.fetch("JWT_ISSUER", "roho-telematics")
JWT_EXP_HOURS = ENV.fetch("JWT_EXP_HOURS", "12").to_i
