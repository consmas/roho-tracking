require "digest"

company = Company.find_or_create_by!(name: "Acme Logistics") do |c|
  c.timezone = "UTC"
end

admin = User.find_or_create_by!(email: "admin@acme.test") do |u|
  u.company = company
  u.password = "Password123!"
  u.role = :admin
end

fleet = Fleet.find_or_create_by!(company: company, name: "Primary Fleet")
vehicle = Vehicle.find_or_create_by!(company: company, plate_number: "ABC-123") do |v|
  v.fleet = fleet
  v.label = "Truck 1"
end

Device.find_or_create_by!(uid: "MDVR-0001") do |d|
  d.company = company
  d.vehicle = vehicle
  d.auth_token_digest = Digest::SHA256.hexdigest(ENV.fetch("DEVICE_AUTH_SHARED_SECRET", "change-me"))
  d.status = :active
end

puts "Seeded company=#{company.name}, user=#{admin.email}, device=MDVR-0001"
