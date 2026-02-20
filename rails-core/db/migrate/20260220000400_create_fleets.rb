class CreateFleets < ActiveRecord::Migration[7.1]
  def change
    create_table :fleets, id: :uuid do |t|
      t.references :company, null: false, type: :uuid, foreign_key: true
      t.string :name, null: false
      t.text :description
      t.timestamps
    end

    add_index :fleets, [:company_id, :name], unique: true
  end
end
