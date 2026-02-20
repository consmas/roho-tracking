class CreateCommands < ActiveRecord::Migration[7.1]
  def change
    create_table :commands, id: :uuid do |t|
      t.references :company, null: false, type: :uuid, foreign_key: true
      t.references :device, null: false, type: :uuid, foreign_key: true
      t.references :user, null: false, type: :uuid, foreign_key: true
      t.string :command_id, null: false
      t.string :command_type, null: false
      t.integer :status, null: false, default: 0
      t.jsonb :payload, null: false, default: {}
      t.datetime :requested_at, null: false
      t.datetime :delivered_at
      t.datetime :acknowledged_at
      t.text :error_message
      t.timestamps
    end

    add_index :commands, :command_id, unique: true
    add_index :commands, [:device_id, :created_at]
    add_index :commands, [:status, :created_at]
  end
end
