//Erreur.h
//Auteur: Tommy Urbain
//Derni�re modification:

#ifndef ERREUR_H
#define ERREUR_H

#include <vector>
#include <string>
#include <QJsonObject>

using std::string;
using std::vector;

class Erreur
{
public:
	//Constructeur et Destructeur
	Erreur();
	Erreur(int noErreur);
	~Erreur();

	string getDescriptionErreur(int noErreur);
	vector<double> noErreurs;

    // Conversion to and from json values.
    void read(const QJsonObject &json);
    void write(QJsonObject &json) const;
};
#endif
