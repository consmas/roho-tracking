class CreateVehicles < ActiveRecord::Migration[7.1]
  def change
    create_table :vehicles, id: :uuid do |t|
      t.references :company, null: false, type: :uuid, foreign_key: true
      t.references :fleet, type: :uuid, foreign_key: true
      t.string :plate_number, null: false
      t.string :vin
      t.string :label
      t.timestamps
    end

    add_index :vehicles, [:company_id, :plate_number], unique: true
    add_index :vehicles, :vin
  end
end
