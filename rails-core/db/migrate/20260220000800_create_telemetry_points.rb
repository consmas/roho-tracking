class CreateTelemetryPoints < ActiveRecord::Migration[7.1]
  def change
    create_table :telemetry_points, id: :uuid do |t|
      t.references :company, null: false, type: :uuid, foreign_key: true
      t.references :vehicle, type: :uuid, foreign_key: true
      t.references :device, null: false, type: :uuid, foreign_key: true
      t.references :event, null: false, type: :uuid, foreign_key: true
      t.decimal :latitude, precision: 10, scale: 7, null: false
      t.decimal :longitude, precision: 10, scale: 7, null: false
      t.decimal :speed_kph, precision: 8, scale: 2
      t.decimal :heading, precision: 6, scale: 2
      t.datetime :fix_time, null: false
      t.jsonb :metadata, null: false, default: {}
      t.timestamps
    end

    add_index :telemetry_points, [:device_id, :fix_time]
    add_index :telemetry_points, [:vehicle_id, :fix_time]
    add_index :telemetry_points, :metadata, using: :gin
  end
end
