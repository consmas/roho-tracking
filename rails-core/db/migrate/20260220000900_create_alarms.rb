class CreateAlarms < ActiveRecord::Migration[7.1]
  def change
    create_table :alarms, id: :uuid do |t|
      t.references :company, null: false, type: :uuid, foreign_key: true
      t.references :device, null: false, type: :uuid, foreign_key: true
      t.references :event, null: false, type: :uuid, foreign_key: true
      t.string :alarm_type, null: false
      t.integer :severity, null: false, default: 1
      t.boolean :acknowledged, null: false, default: false
      t.datetime :occurred_at, null: false
      t.jsonb :details, null: false, default: {}
      t.timestamps
    end

    add_index :alarms, [:device_id, :occurred_at]
    add_index :alarms, [:acknowledged, :occurred_at]
    add_index :alarms, :details, using: :gin
  end
end
