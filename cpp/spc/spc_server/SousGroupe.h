//Analyse_Form.h
//Auteur: Tommy Urbain
//Derni�re modification:

#ifndef SOUS_GROUPE_H
#define SOUS_GROUPE_H

#include <vector>

// Serialysation to json.
#include <QJsonArray>
#include <QJsonObject>

class SousGroupe
{
	//attributs priv�es
	double range;
	double moyenne;
	bool state;

public:
	//Constructeur et destructeur
	SousGroupe();
	~SousGroupe();
	std::vector<double> donnees;

	//Accesseurs
	double getRange();
	double getMoyenne();
	bool getState();

	//Mutateurs
	void setRange(double range);
	void setMoyenne(double moyenne);
	void setState(bool state);

    // Conversion to and from json values.
    void read(const QJsonObject &json);
    void write(QJsonObject &json) const;
};
#endif
