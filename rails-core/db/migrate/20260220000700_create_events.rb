class CreateEvents < ActiveRecord::Migration[7.1]
  def change
    create_table :events, id: :uuid do |t|
      t.references :company, null: false, type: :uuid, foreign_key: true
      t.references :device, null: false, type: :uuid, foreign_key: true
      t.string :event_id, null: false
      t.string :event_type, null: false
      t.datetime :occurred_at, null: false
      t.jsonb :data, null: false, default: {}
      t.text :raw
      t.timestamps
    end

    add_index :events, :event_id, unique: true
    add_index :events, [:device_id, :occurred_at]
    add_index :events, [:event_type, :occurred_at]
    add_index :events, :data, using: :gin
  end
end
