class CreateGeofences < ActiveRecord::Migration[7.1]
  def change
    create_table :geofences, id: :uuid do |t|
      t.references :company, null: false, type: :uuid, foreign_key: true
      t.string :name, null: false
      t.jsonb :geometry, null: false, default: {}
      t.boolean :active, null: false, default: true
      t.timestamps
    end

    add_index :geofences, [:company_id, :active]
    add_index :geofences, :geometry, using: :gin
  end
end
