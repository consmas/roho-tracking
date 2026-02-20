class CreateDeviceSessions < ActiveRecord::Migration[7.1]
  def change
    create_table :device_sessions, id: :uuid do |t|
      t.references :company, null: false, type: :uuid, foreign_key: true, index: false
      t.references :device, null: false, type: :uuid, foreign_key: true, index: false
      t.string :gateway_instance_id, null: false
      t.datetime :last_heartbeat_at, null: false
      t.timestamps
    end

    add_index :device_sessions, :company_id
    add_index :device_sessions, :device_id, unique: true
    add_index :device_sessions, [:gateway_instance_id, :last_heartbeat_at]
  end
end
