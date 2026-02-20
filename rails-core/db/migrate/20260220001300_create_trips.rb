class CreateTrips < ActiveRecord::Migration[7.1]
  def change
    create_table :trips, id: :uuid do |t|
      t.references :company, null: false, type: :uuid, foreign_key: true
      t.references :vehicle, null: false, type: :uuid, foreign_key: true
      t.references :device, null: false, type: :uuid, foreign_key: true
      t.datetime :started_at, null: false
      t.datetime :ended_at, null: false
      t.decimal :distance_km, precision: 10, scale: 3, null: false
      t.decimal :max_speed_kph, precision: 8, scale: 2
      t.jsonb :summary, null: false, default: {}
      t.timestamps
    end

    add_index :trips, [:vehicle_id, :started_at]
    add_index :trips, [:device_id, :started_at]
  end
end
